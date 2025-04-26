package clickhouse

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mr-karan/logchef/pkg/models"
)

// LogQueryParams defines parameters for querying logs.
type LogQueryParams struct {
	Limit  int
	RawSQL string
}

// LogQueryResult represents the structured result of a log query.
type LogQueryResult struct {
	Data    []map[string]interface{} `json:"data"`
	Stats   models.QueryStats        `json:"stats"`
	Columns []models.ColumnInfo      `json:"columns"`
}

// TimeWindow represents the desired granularity for time-based aggregations.
type TimeWindow string

const (
	// Second-based windows
	TimeWindow1s  TimeWindow = "1s"  // 1 second
	TimeWindow5s  TimeWindow = "5s"  // 5 seconds
	TimeWindow10s TimeWindow = "10s" // 10 seconds
	TimeWindow15s TimeWindow = "15s" // 15 seconds
	TimeWindow30s TimeWindow = "30s" // 30 seconds

	// Minute-based windows
	TimeWindow1m  TimeWindow = "1m"  // 1 minute
	TimeWindow5m  TimeWindow = "5m"  // 5 minutes
	TimeWindow10m TimeWindow = "10m" // 10 minutes
	TimeWindow15m TimeWindow = "15m" // 15 minutes
	TimeWindow30m TimeWindow = "30m" // 30 minutes

	// Hour-based windows
	TimeWindow1h  TimeWindow = "1h"  // 1 hour
	TimeWindow2h  TimeWindow = "2h"  // 2 hours
	TimeWindow3h  TimeWindow = "3h"  // 3 hours
	TimeWindow6h  TimeWindow = "6h"  // 6 hours
	TimeWindow12h TimeWindow = "12h" // 12 hours
	TimeWindow24h TimeWindow = "24h" // 24 hours
)

// LogContextParams defines parameters for fetching logs around a specific target time.
type LogContextParams struct {
	TargetTime  time.Time
	BeforeLimit int
	AfterLimit  int
}

// LogContextResult holds the logs retrieved before, at, and after the target time.
type LogContextResult struct {
	BeforeLogs []map[string]interface{}
	TargetLogs []map[string]interface{} // Logs exactly at the target time.
	AfterLogs  []map[string]interface{}
	Stats      models.QueryStats
}

// HistogramParams defines parameters for generating histogram data.
type HistogramParams struct {
	StartTime time.Time
	EndTime   time.Time
	Window    TimeWindow
	Query     string // Raw SQL query to use as base for histogram
	GroupBy   string // Optional: Field to group by for segmented histograms.
	Timezone  string // Optional: Timezone identifier for time-based operations.
}

// HistogramData represents a single time bucket and its log count in a histogram.
type HistogramData struct {
	Bucket     time.Time `json:"bucket"`      // Start time of the bucket.
	LogCount   int       `json:"log_count"`   // Number of logs in the bucket.
	GroupValue string    `json:"group_value"` // Value of the group for grouped histograms.
}

// HistogramResult holds the complete histogram data and its granularity.
type HistogramResult struct {
	Granularity string          `json:"granularity"` // The time window used (e.g., "5m").
	Data        []HistogramData `json:"data"`
}

// Helper function to find the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Helper function to extract non-time WHERE conditions from SQL query
// It specifically tries to ignore `timestamp >=`, `timestamp <=`, and `timestamp BETWEEN` clauses.
func extractNonTimeWhereConditions(sql string, timestampField string) string {
	// 1. Extract the full WHERE clause content first
	wherePattern := `(?i)WHERE\s+(.*?)(?:\s+(?:GROUP\s+BY|ORDER\s+BY|LIMIT|HAVING|OFFSET|UNION|INTERSECT|EXCEPT|$))`
	re := regexp.MustCompile(wherePattern)
	matches := re.FindStringSubmatch(sql)

	fullWhereClause := ""
	if len(matches) > 1 {
		fullWhereClause = strings.TrimSpace(matches[1])
	} else {
		// If no WHERE keyword, but might contain conditions, assume the whole string is conditions
		operatorPattern := `(?i)(=|!=|<>|<|>|<=|>=|LIKE|IN\s*\(|NOT IN\s*\(|BETWEEN)`
		if regexp.MustCompile(operatorPattern).MatchString(sql) {
			fullWhereClause = strings.TrimSpace(sql)
		}
	}

	if fullWhereClause == "" {
		return "" // No conditions found
	}

	// 2. Split conditions by AND/OR (simplistic split, might break with nested parentheses)
	// We need to preserve the AND/OR structure.
	// Regex to split by AND/OR while keeping them as delimiters
	// This regex uses lookahead/lookbehind to split correctly
	splitPattern := `(?i)\s+(AND|OR)\s+`
	splitRegex := regexp.MustCompile(splitPattern)

	// Find all delimiters and their positions
	delimiterIndices := splitRegex.FindAllStringIndex(fullWhereClause, -1)
	delimiterMatches := splitRegex.FindAllString(fullWhereClause, -1)

	conditions := []string{}
	lastIndex := 0
	for i, indices := range delimiterIndices {
		// Add the condition before the delimiter
		conditions = append(conditions, strings.TrimSpace(fullWhereClause[lastIndex:indices[0]]))
		// Add the delimiter itself
		conditions = append(conditions, strings.TrimSpace(delimiterMatches[i]))
		lastIndex = indices[1]
	}
	// Add the last condition after the final delimiter
	conditions = append(conditions, strings.TrimSpace(fullWhereClause[lastIndex:]))

	// 3. Filter out time-related conditions
	filteredConditions := []string{}
	timestampPattern := fmt.Sprintf(`(?i)^\s*%s\s*(>=|<=|BETWEEN)`, regexp.QuoteMeta(timestampField))
	timestampRegex := regexp.MustCompile(timestampPattern)

	// Reconstruct the query, skipping time conditions but keeping AND/OR
	keepNextOperator := true // Start by potentially keeping the first condition
	for i := 0; i < len(conditions); i++ {
		isCondition := (i%2 == 0) // Even indices are conditions, odd are operators (AND/OR)

		if isCondition {
			condition := conditions[i]
			if !timestampRegex.MatchString(condition) {
				// It's not a time condition, keep it
				if !keepNextOperator && len(filteredConditions) > 0 {
					// Need to add the preceding operator (AND/OR) if this isn't the first condition kept
					filteredConditions = append(filteredConditions, conditions[i-1])
				}
				filteredConditions = append(filteredConditions, condition)
				keepNextOperator = false // The next operator should be kept
			} else {
				// It IS a time condition, skip it. Also skip the *next* operator if one exists
				keepNextOperator = true
			}
		} else { // It's an operator (AND/OR)
			// Operator logic is handled when processing the *next* condition
		}
	}

	// Handle edge case: if the *last* condition was a time condition, the loop might end
	// without removing a preceding AND/OR that is now dangling.
	if len(filteredConditions) > 0 {
		lastElement := filteredConditions[len(filteredConditions)-1]
		if strings.EqualFold(lastElement, "AND") || strings.EqualFold(lastElement, "OR") {
			filteredConditions = filteredConditions[:len(filteredConditions)-1]
		}
	}

	return strings.Join(filteredConditions, " ")
}

// GetHistogramData generates histogram data by grouping log counts into time buckets.
// It uses the provided raw SQL as the base query and applies time window aggregation.
func (c *Client) GetHistogramData(ctx context.Context, tableName, timestampField string, params HistogramParams) (*HistogramResult, error) {
	// Get timezone or default to UTC
	timezone := params.Timezone
	if timezone == "" {
		timezone = "UTC"
	}

	// Convert TimeWindow to the appropriate ClickHouse interval function
	var intervalFunc string
	switch params.Window {
	case TimeWindow1s:
		intervalFunc = fmt.Sprintf("toStartOfSecond(%s, '%s')", timestampField, timezone)
	case TimeWindow5s, TimeWindow10s, TimeWindow15s, TimeWindow30s:
		// For custom second intervals, use toStartOfInterval
		seconds := strings.TrimSuffix(string(params.Window), "s")
		intervalFunc = fmt.Sprintf("toStartOfInterval(%s, INTERVAL %s SECOND, '%s')", timestampField, seconds, timezone)
	case TimeWindow1m:
		intervalFunc = fmt.Sprintf("toStartOfMinute(%s, '%s')", timestampField, timezone)
	case TimeWindow5m:
		intervalFunc = fmt.Sprintf("toStartOfFiveMinute(%s, '%s')", timestampField, timezone)
	case TimeWindow10m, TimeWindow15m, TimeWindow30m:
		// For custom minute intervals, use toStartOfInterval
		minutes := strings.TrimSuffix(string(params.Window), "m")
		intervalFunc = fmt.Sprintf("toStartOfInterval(%s, INTERVAL %s MINUTE, '%s')", timestampField, minutes, timezone)
	case TimeWindow1h:
		intervalFunc = fmt.Sprintf("toStartOfHour(%s, '%s')", timestampField, timezone)
	case TimeWindow2h, TimeWindow3h, TimeWindow6h, TimeWindow12h, TimeWindow24h:
		// For custom hour intervals, use toStartOfInterval
		hours := strings.TrimSuffix(string(params.Window), "h")
		intervalFunc = fmt.Sprintf("toStartOfInterval(%s, INTERVAL %s HOUR, '%s')", timestampField, hours, timezone)
	default:
		return nil, fmt.Errorf("invalid time window: %s", params.Window)
	}

	// Process the raw SQL query if provided
	baseQuery := ""
	if params.Query != "" {
		// Use the query builder to remove LIMIT clause
		qb := NewQueryBuilder(tableName)
		var err error
		baseQuery, err = qb.RemoveLimitClause(params.Query)
		if err != nil {
			return nil, fmt.Errorf("failed to process base query: %w", err)
		}
	} else {
		// Fallback to a simple query if none provided
		baseQuery = fmt.Sprintf("SELECT * FROM %s", tableName)
	}

	// Construct the histogram query using CTE
	var query string
	if params.GroupBy != "" && strings.TrimSpace(params.GroupBy) != "" {
		// Histogram with grouping - find top N groups
		query = fmt.Sprintf(`
			WITH 
				top_groups AS (
					SELECT
						%s AS group_value,
						count(*) AS total_logs
					FROM (%s) AS raw_logs
					GROUP BY group_value
					ORDER BY total_logs DESC
					LIMIT 10
				)
			SELECT
				%s AS bucket,
				%s AS group_value,
				count(*) AS log_count
			FROM (%s) AS raw_logs
			WHERE %s GLOBAL IN (SELECT group_value FROM top_groups)
			GROUP BY
				bucket,
				group_value
			ORDER BY
				bucket ASC,
				log_count DESC
		`, params.GroupBy, baseQuery, intervalFunc, params.GroupBy, baseQuery, params.GroupBy)
	} else {
		// Standard histogram without grouping
		query = fmt.Sprintf(`
			SELECT
				%s AS bucket,
				count(*) AS log_count
			FROM (%s) AS raw_logs
			GROUP BY bucket
			ORDER BY bucket ASC
		`, intervalFunc, baseQuery)
	}

	// Execute the query
	result, err := c.Query(ctx, query)
	if err != nil {
		c.logger.Error("failed to execute histogram query", "error", err, "table", tableName)
		return nil, fmt.Errorf("failed to execute histogram query: %w", err)
	}

	// Parse results into HistogramData
	var results []HistogramData

	for _, row := range result.Logs {
		bucket, okB := row["bucket"].(time.Time)
		countVal, okC := row["log_count"] // Type can vary (uint64, int64, etc.)

		if !okB || !okC {
			c.logger.Warn("unexpected type in histogram row, skipping",
				"bucket_val", row["bucket"],
				"count_val", row["log_count"])
			continue
		}

		// Safely convert count to int
		count := 0
		switch v := countVal.(type) {
		case uint64:
			count = int(v)
		case int64:
			count = int(v)
		case int:
			count = v
		case float64:
			count = int(v)
		default:
			c.logger.Warn("unexpected numeric type for log_count in histogram row",
				"type", fmt.Sprintf("%T", countVal))
			continue
		}

		groupValueStr := ""
		if params.GroupBy != "" {
			groupVal, okG := row["group_value"]
			if !okG {
				c.logger.Warn("missing group_value in histogram row")
				continue
			}

			// Convert group value to string
			switch v := groupVal.(type) {
			case string:
				groupValueStr = v
			case nil:
				groupValueStr = "null"
			default:
				groupValueStr = fmt.Sprintf("%v", v)
			}
		}

		results = append(results, HistogramData{
			Bucket:     bucket,
			LogCount:   count,
			GroupValue: groupValueStr,
		})
	}

	return &HistogramResult{
		Granularity: string(params.Window),
		Data:        results,
	}, nil
}
