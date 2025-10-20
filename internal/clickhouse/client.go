package clickhouse

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mr-karan/logchef/internal/metrics"
	"github.com/mr-karan/logchef/pkg/models"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Default values for query execution
const (
	// DefaultQueryTimeout is the default max_execution_time in seconds if not specified
	DefaultQueryTimeout = 60
	// MaxQueryTimeout is the maximum allowed timeout to prevent resource abuse
	MaxQueryTimeout = 300 // 5 minutes
)

// Client represents a connection to a ClickHouse database using the native protocol.
// It provides methods for executing queries and retrieving metadata.
type Client struct {
	conn       driver.Conn // Underlying ClickHouse native connection.
	logger     *slog.Logger
	queryHooks []QueryHook         // Hooks to execute before/after queries.
	mu         sync.Mutex          // Protects shared resources within the client if any
	opts       *clickhouse.Options // Stores connection options for reconnection
	sourceID   string              // Source ID for metrics tracking
	source     *models.Source      // Source model for metrics with meaningful labels
	metrics    *metrics.ClickHouseMetrics
}

// ClientOptions holds configuration for establishing a new ClickHouse client connection.
type ClientOptions struct {
	Host     string                 // Hostname or IP address.
	Database string                 // Target database name.
	Username string                 // Username for authentication.
	Password string                 // Password for authentication.
	Settings map[string]interface{} // Additional ClickHouse settings (e.g., max_execution_time).
	SourceID string                 // Source ID for metrics tracking.
	Source   *models.Source         // Source model for enhanced metrics.
}

// ExtendedColumnInfo provides detailed column metadata, including nullability,
// primary key status, default expressions, and comments, supplementing models.ColumnInfo.
type ExtendedColumnInfo struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	IsNullable        bool   `json:"is_nullable"`
	IsPrimaryKey      bool   `json:"is_primary_key"`
	DefaultExpression string `json:"default_expression,omitempty"`
	Comment           string `json:"comment,omitempty"`
}

// TableInfo represents comprehensive metadata about a ClickHouse table, including
// engine details, column definitions (basic and extended), sorting keys, and the CREATE statement.
type TableInfo struct {
	Database     string               `json:"database"`
	Name         string               `json:"name"`
	Engine       string               `json:"engine"`                  // e.g., "MergeTree", "Distributed"
	EngineParams []string             `json:"engine_params,omitempty"` // Parameters extracted from engine_full.
	Columns      []models.ColumnInfo  `json:"columns"`                 // Basic column info (Name, Type).
	ExtColumns   []ExtendedColumnInfo `json:"ext_columns,omitempty"`   // Detailed column info.
	SortKeys     []string             `json:"sort_keys"`               // Parsed sorting key columns.
	CreateQuery  string               `json:"create_query,omitempty"`  // Full CREATE TABLE statement.
}

// NewClient establishes a new connection to a ClickHouse server using the native protocol.
// It takes connection options and a logger, creates the connection, and returns a Client instance.
// Note: This does not automatically verify the connection with a ping - callers should do that if needed.
func NewClient(opts ClientOptions, logger *slog.Logger) (*Client, error) {
	// Ensure host includes the native protocol port (default 9000) if not specified.
	host := opts.Host
	if !strings.Contains(host, ":") {
		host = host + ":9000"
	}

	options := &clickhouse.Options{
		Addr: []string{host},
		Auth: clickhouse.Auth{
			Database: opts.Database,
			Username: opts.Username,
			Password: opts.Password,
		},
		TLS: &tls.Config{},
		Settings: clickhouse.Settings{
			// Default settings.
			"max_execution_time": 60,
		},
		DialTimeout: 10 * time.Second,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		Protocol: clickhouse.Native,
	}

	// Apply any additional user-provided settings.
	if opts.Settings != nil {
		for k, v := range opts.Settings {
			options.Settings[k] = v
		}
	}

	logger.Debug("creating clickhouse connection",
		"host", host,
		"database", opts.Database,
		"protocol", "native",
	)

	conn, err := clickhouse.Open(options)
	if err != nil {
		return nil, fmt.Errorf("creating clickhouse connection: %w", err)
	}

	client := &Client{
		conn:       conn,
		logger:     logger,
		queryHooks: []QueryHook{}, // Initialize hooks slice.
		opts:       options,
		sourceID:   opts.SourceID,
		source:     opts.Source,
	}

	// Apply a default hook for basic query logging.
	client.AddQueryHook(NewLogQueryHook(logger, false)) // Verbose logging disabled by default.

	// Add metrics hook if source is provided
	if opts.Source != nil {
		client.AddQueryHook(metrics.NewMetricsQueryHook(opts.Source))
		client.metrics = metrics.NewClickHouseMetrics(opts.Source)
	}

	return client, nil
}

// AddQueryHook registers a hook to be executed before and after queries run by this client.
func (c *Client) AddQueryHook(hook QueryHook) {
	c.queryHooks = append(c.queryHooks, hook)
}

// executeQueryWithHooks wraps the execution of a query function (`fn`)
// with the registered BeforeQuery and AfterQuery hooks.
func (c *Client) executeQueryWithHooks(ctx context.Context, query string, fn func(context.Context) error) error {
	var err error
	start := time.Now()

	// Execute BeforeQuery hooks.
	for _, hook := range c.queryHooks {
		ctx, err = hook.BeforeQuery(ctx, query)
		if err != nil {
			// If a hook fails, log and return the error immediately.
			c.logger.Error("query hook BeforeQuery failed", "hook", fmt.Sprintf("%T", hook), "error", err)
			return fmt.Errorf("BeforeQuery hook failed: %w", err)
		}
	}

	// Execute the actual query function.
	err = fn(ctx) // This might be conn.Query, conn.Exec, etc.
	duration := time.Since(start)

	// Execute AfterQuery hooks, regardless of query success/failure.
	for _, hook := range c.queryHooks {
		// Hooks should ideally handle logging internally if needed.
		hook.AfterQuery(ctx, query, err, duration)
	}

	return err // Return the error from the query function itself.
}

// Query executes a SELECT query, processes the results, and applies query hooks.
// It automatically handles DDL statements by calling execDDL.
// The params argument is now unused but kept for potential future structured query building.
func (c *Client) Query(ctx context.Context, query string /* params LogQueryParams - Removed */) (*models.QueryResult, error) {
	return c.QueryWithTimeout(ctx, query, nil)
}

// QueryWithTimeout executes a SELECT query with a timeout setting.
// The timeoutSeconds parameter is required and will always be applied.
func (c *Client) QueryWithTimeout(ctx context.Context, query string, timeoutSeconds *int) (*models.QueryResult, error) {
	start := time.Now()          // Used for calculating total duration including hook overhead.
	queryStartTime := time.Now() // Separate timer for actual DB execution
	var queryDuration time.Duration

	// Start query metrics tracking
	var queryHelper *metrics.QueryMetricsHelper
	if c.metrics != nil {
		queryType := metrics.DetermineQueryType(query)
		queryHelper = c.metrics.StartQuery(queryType, nil) // User context not available in client
	}

	// Ensure timeout is provided (should always be the case now)
	if timeoutSeconds == nil {
		defaultTimeout := DefaultQueryTimeout
		timeoutSeconds = &defaultTimeout
	}

	defer func() {
		c.logger.Debug("query processing complete",
			"duration_ms", time.Since(start).Milliseconds(),
			"query_length", len(query),
			"timeout_seconds", *timeoutSeconds,
		)
	}()

	// Delegate DDL statements (CREATE, ALTER, DROP, etc.) to execDDL.
	if isDDLStatement(query) {
		return c.execDDLWithTimeout(ctx, query, timeoutSeconds)
	}

	var rows driver.Rows
	var resultData []map[string]interface{}
	var columnsInfo []models.ColumnInfo

	// Execute the core query logic within the hook wrapper.
	err := c.executeQueryWithHooks(ctx, query, func(hookCtx context.Context) error {
		var queryErr error
		queryStartTime = time.Now() // Reset timer before execution

		// Always apply timeout setting
		hookCtx = clickhouse.Context(hookCtx, clickhouse.WithSettings(clickhouse.Settings{
			"max_execution_time": *timeoutSeconds,
		}))
		c.logger.Debug("applying query timeout", "timeout_seconds", *timeoutSeconds)

		rows, queryErr = c.conn.Query(hookCtx, query)
		if queryErr != nil {
			return queryErr
		}

		// Close rows when we're done processing them
		defer func() {
			if rows != nil {
				rows.Close()
			}
		}()

		// Get column names and types.
		columnTypes := rows.ColumnTypes()
		columnsInfo = make([]models.ColumnInfo, len(columnTypes)) // Use new name
		scanDest := make([]interface{}, len(columnTypes))         // Prepare scan destinations.
		for i, ct := range columnTypes {
			columnsInfo[i] = models.ColumnInfo{
				Name: ct.Name(),
				Type: ct.DatabaseTypeName(),
			}
			// Use reflection to create pointers of the correct underlying type for Scan.
			scanDest[i] = reflect.New(ct.ScanType()).Interface()
		}

		// Process rows.
		resultData = make([]map[string]interface{}, 0) // Initialize slice.
		for rows.Next() {
			if err := rows.Scan(scanDest...); err != nil {
				return fmt.Errorf("scanning row: %w", err)
			}

			rowMap := make(map[string]interface{})
			for i, col := range columnsInfo { // Use new name
				// Dereference the pointer to get the actual scanned value.
				rowMap[col.Name] = reflect.ValueOf(scanDest[i]).Elem().Interface()
			}
			resultData = append(resultData, rowMap)
		}
		queryDuration = time.Since(queryStartTime) // Capture DB execution duration

		// Check for errors during row iteration.
		return rows.Err()
	})

	// Complete metrics tracking
	if queryHelper != nil {
		success := err == nil
		rowsReturned := int64(-1)
		if success && resultData != nil {
			rowsReturned = int64(len(resultData))
		}
		errorType := metrics.DetermineErrorType(err)
		timedOut := false // TODO: better timeout detection
		queryHelper.Finish(success, rowsReturned, errorType, timedOut)
	}

	// Handle errors from either query execution or row processing.
	if err != nil {
		return nil, fmt.Errorf("executing query or processing results: %w", err)
	}

	// Construct the final result.
	queryResult := &models.QueryResult{
		Logs:    resultData,
		Columns: columnsInfo,
		Stats: models.QueryStats{
			RowsRead:        len(resultData), // Use length of returned data as approximation
			BytesRead:       0,               // Cannot reliably get BytesRead currently
			ExecutionTimeMs: float64(queryDuration.Milliseconds()),
		},
	}

	return queryResult, nil
}

// execDDL executes a DDL statement (e.g., CREATE, ALTER, DROP) using hooks.
// It returns an empty QueryResult on success.
func (c *Client) execDDL(ctx context.Context, query string) (*models.QueryResult, error) {
	return c.execDDLWithTimeout(ctx, query, nil)
}

// execDDLWithTimeout executes a DDL statement with a timeout setting.
// The timeoutSeconds parameter is required and will always be applied.
func (c *Client) execDDLWithTimeout(ctx context.Context, query string, timeoutSeconds *int) (*models.QueryResult, error) {
	start := time.Now()

	// Ensure timeout is provided (should always be the case now)
	if timeoutSeconds == nil {
		defaultTimeout := DefaultQueryTimeout
		timeoutSeconds = &defaultTimeout
	}

	err := c.executeQueryWithHooks(ctx, query, func(hookCtx context.Context) error {
		// Always apply timeout setting
		hookCtx = clickhouse.Context(hookCtx, clickhouse.WithSettings(clickhouse.Settings{
			"max_execution_time": *timeoutSeconds,
		}))
		c.logger.Debug("applying DDL query timeout", "timeout_seconds", *timeoutSeconds)

		return c.conn.Exec(hookCtx, query)
	})

	if err != nil {
		return nil, fmt.Errorf("executing DDL query: %w", err)
	}

	// Return empty result for DDL statements.
	return &models.QueryResult{
		Logs:    []map[string]interface{}{},
		Columns: []models.ColumnInfo{},
		Stats: models.QueryStats{
			RowsRead:        0,
			ExecutionTimeMs: float64(time.Since(start).Milliseconds()),
		},
	}, nil
}

// isDDLStatement checks if a query string likely represents a DDL statement.
func isDDLStatement(query string) bool {
	// Simple prefix check after trimming and uppercasing.
	upperQuery := strings.ToUpper(strings.TrimSpace(query))
	ddlPrefixes := []string{"CREATE", "ALTER", "DROP", "TRUNCATE", "RENAME"}
	for _, prefix := range ddlPrefixes {
		if strings.HasPrefix(upperQuery, prefix) {
			return true
		}
	}
	return false
}

// Close terminates the underlying database connection with a timeout.
func (c *Client) Close() error {
	c.logger.Info("closing clickhouse client connection")

	// Create a timeout context for the close operation
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Create a channel to signal completion
	done := make(chan error, 1)

	// Close the connection in a goroutine
	go func() {
		done <- c.conn.Close()
	}()

	// Wait for close to complete or timeout
	select {
	case err := <-done:
		// Connection closed normally
		return err
	case <-ctx.Done():
		// Timeout occurred
		c.logger.Warn("timeout while closing clickhouse connection, abandoning")
		return fmt.Errorf("timeout while closing connection")
	}
}

// Reconnect attempts to re-establish the connection to the ClickHouse server.
// This is useful for recovering from connection failures during health checks.
func (c *Client) Reconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	success := false
	defer func() {
		if c.metrics != nil {
			c.metrics.RecordReconnection(success)
			c.metrics.UpdateConnectionStatus(success)
		}
	}()

	// Only attempt reconnect if connection exists but is failing
	if c.conn != nil {
		// Try to close the existing connection first with a timeout
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer closeCancel()

		closeComplete := make(chan struct{})
		go func() {
			_ = c.conn.Close() // Ignore close errors
			close(closeComplete)
		}()

		// Wait for close to complete or timeout
		select {
		case <-closeComplete:
			// Successfully closed
			c.logger.Debug("successfully closed old connection for reconnect")
		case <-closeCtx.Done():
			// Timeout occurred
			c.logger.Warn("timeout closing old connection for reconnect, proceeding anyway")
		}
	}

	// Use stored connection options
	if c.opts == nil {
		return fmt.Errorf("missing connection options for reconnect")
	}

	// Create a new connection with the same settings
	newConn, err := clickhouse.Open(c.opts)
	if err != nil {
		return fmt.Errorf("reconnecting to clickhouse: %w", err)
	}

	// Test the new connection with a short timeout
	pingCtx, pingCancel := context.WithTimeout(ctx, 3*time.Second)
	defer pingCancel()

	if err := newConn.Ping(pingCtx); err != nil {
		// Clean up failed connection with timeout
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer closeCancel()

		go func() {
			_ = newConn.Close() // Clean up failed connection
			close(make(chan struct{}))
		}()

		// Just wait for timeout - we don't care about the result
		<-closeCtx.Done()

		return fmt.Errorf("ping after reconnect failed: %w", err)
	}

	// Replace the connection
	c.conn = newConn
	success = true
	c.logger.Info("successfully reconnected to clickhouse")
	return nil
}

// GetTableInfo retrieves detailed metadata about a table, including handling
// for Distributed tables by inspecting the underlying local table.
func (c *Client) GetTableInfo(ctx context.Context, database, table string) (*TableInfo, error) {
	start := time.Now()
	defer func() {
		c.logger.Debug("table info query completed",
			"duration_ms", time.Since(start).Milliseconds(),
			"database", database,
			"table", table,
		)
	}()

	// First, get the base info (engine, columns, create statement) for the specified table.
	baseInfo, err := c.getBaseTableInfo(ctx, database, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get base table info: %w", err)
	}

	// If it's a Distributed engine table, fetch metadata from the underlying local table.
	if baseInfo.Engine == "Distributed" && len(baseInfo.EngineParams) >= 3 {
		return c.handleDistributedTable(ctx, baseInfo)
	}

	// If it's a MergeTree family table, attempt to get sorting keys.
	if strings.Contains(baseInfo.Engine, "MergeTree") {
		sortKeys, err := c.getSortKeys(ctx, database, table)
		if err != nil {
			// Log failure but don't fail the entire operation.
			c.logger.Warn("failed to get sort keys", "error", err, "database", database, "table", table)
		} else {
			baseInfo.SortKeys = sortKeys
		}
	}

	return baseInfo, nil
}

// getBaseTableInfo retrieves the fundamental metadata for a table from system tables.
func (c *Client) getBaseTableInfo(ctx context.Context, database, table string) (*TableInfo, error) {
	engine, params, createQuery, err := c.getTableEngine(ctx, database, table)
	if err != nil {
		return nil, err // Error getting engine details is fatal here.
	}

	columns, err := c.getColumns(ctx, database, table)
	if err != nil {
		return nil, err // Error getting basic columns is fatal here.
	}

	// Extended column info is optional; log errors but don't fail.
	// Try to get extended columns, but handle version compatibility gracefully.
	extColumns, err := c.getExtendedColumns(ctx, database, table)
	if err != nil {
		c.logger.Warn("failed to get extended column info",
			"error", err,
			"database", database,
			"table", table,
		)
		// Set to nil to indicate extended columns are not available
		extColumns = nil
	}

	return &TableInfo{
		Database:     database,
		Name:         table,
		Engine:       engine,
		EngineParams: params,
		CreateQuery:  createQuery,
		Columns:      columns,
		ExtColumns:   extColumns,
		// SortKeys added later if applicable.
	}, nil
}

// getExtendedColumns retrieves detailed column metadata from system.columns.
// This function handles version compatibility by checking available columns.
func (c *Client) getExtendedColumns(ctx context.Context, database, table string) ([]ExtendedColumnInfo, error) {
	// Use a simpler query that works across more ClickHouse versions
	// The is_nullable column is not available in all versions
	query := `
		SELECT
			name, type,
			is_in_primary_key,
			default_expression,
			comment
		FROM system.columns
		WHERE database = ? AND table = ?
		ORDER BY position
	`
	var rows driver.Rows
	var err error

	// Use hook wrapper for consistency, though less critical for metadata queries.
	err = c.executeQueryWithHooks(ctx, query, func(hookCtx context.Context) error {
		rows, err = c.conn.Query(hookCtx, query, database, table)
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("failed to query extended columns: %w", err)
	}
	defer rows.Close()

	var columns []ExtendedColumnInfo
	for rows.Next() {
		var col ExtendedColumnInfo
		err := rows.Scan(
			&col.Name, &col.Type,
			&col.IsPrimaryKey,
			&col.DefaultExpression,
			&col.Comment,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan extended column: %w", err)
		}
		// Determine nullability from the type string since is_nullable column may not be available
		col.IsNullable = strings.HasPrefix(col.Type, "Nullable(")
		columns = append(columns, col)
	}
	return columns, rows.Err() // Return any error encountered during iteration.
}

// getTableEngine retrieves the table engine, full engine string, and CREATE statement.
func (c *Client) getTableEngine(ctx context.Context, database, table string) (string, []string, string, error) {
	query := `
		SELECT engine, engine_full, create_table_query
		FROM system.tables
		WHERE database = ? AND name = ?
	`
	var rows driver.Rows
	var err error

	err = c.executeQueryWithHooks(ctx, query, func(hookCtx context.Context) error {
		rows, err = c.conn.Query(hookCtx, query, database, table)
		return err
	})

	if err != nil {
		return "", nil, "", fmt.Errorf("failed to query table engine: %w", err)
	}
	defer rows.Close()

	var engine, engineFull, createQuery string
	if rows.Next() {
		if err := rows.Scan(&engine, &engineFull, &createQuery); err != nil {
			return "", nil, "", fmt.Errorf("failed to scan table engine: %w", err)
		}
	} else {
		// If no rows returned, the table likely doesn't exist.
		return "", nil, "", fmt.Errorf("table %s.%s not found in system.tables", database, table)
	}
	if err := rows.Err(); err != nil {
		return "", nil, "", fmt.Errorf("error iterating table engine results: %w", err)
	}

	// Skip parsing engine_full since it contains the entire engine clause (PARTITION BY, ORDER BY, TTL)
	// rather than just constructor parameters. The actual schema details are available in other fields.
	// Only parse constructor params if we specifically need them for Distributed engines.
	var params []string
	if strings.HasPrefix(engine, "Distributed") {
		params = parseEngineParams(engineFull)
	}
	return engine, params, createQuery, nil
}

// getColumns retrieves basic column name and type information.
func (c *Client) getColumns(ctx context.Context, database, table string) ([]models.ColumnInfo, error) {
	query := `
		SELECT name, type
		FROM system.columns
		WHERE database = ? AND table = ?
		ORDER BY position
	`
	var rows driver.Rows
	var err error

	err = c.executeQueryWithHooks(ctx, query, func(hookCtx context.Context) error {
		rows, err = c.conn.Query(hookCtx, query, database, table)
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %w", err)
	}
	defer rows.Close()

	var columns []models.ColumnInfo
	for rows.Next() {
		var col models.ColumnInfo
		if err := rows.Scan(&col.Name, &col.Type); err != nil {
			return nil, fmt.Errorf("failed to scan column: %w", err)
		}
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

// getSortKeys retrieves the sorting key expression for MergeTree family tables.
func (c *Client) getSortKeys(ctx context.Context, database, table string) ([]string, error) {
	// This query assumes the table engine is MergeTree compatible.
	query := `
		SELECT sorting_key
		FROM system.tables
		WHERE database = ? AND name = ?
	`
	var rows driver.Rows
	var err error

	err = c.executeQueryWithHooks(ctx, query, func(hookCtx context.Context) error {
		rows, err = c.conn.Query(hookCtx, query, database, table)
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("failed to query sort keys: %w", err)
	}
	defer rows.Close()

	var sortKeys string
	if rows.Next() {
		if err := rows.Scan(&sortKeys); err != nil {
			return nil, fmt.Errorf("failed to scan sort keys: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sort key results: %w", err)
	}

	// Parse the potentially complex sorting_key string into individual column names.
	return parseSortKeys(sortKeys), nil
}

// handleDistributedTable fetches metadata from the underlying local table
// referenced by a Distributed table engine.
func (c *Client) handleDistributedTable(ctx context.Context, base *TableInfo) (*TableInfo, error) {
	if len(base.EngineParams) < 3 {
		// Distributed engine string is malformed or unexpected.
		c.logger.Warn("distributed table has insufficient engine parameters", "params", base.EngineParams)
		return base, nil // Return base info as fallback.
	}

	// Extract cluster, local database, and local table names from engine parameters.
	cluster := base.EngineParams[0]
	localDB := base.EngineParams[1]
	localTable := base.EngineParams[2]

	c.logger.Debug("resolving distributed table metadata",
		"distributed_table", fmt.Sprintf("%s.%s", base.Database, base.Name),
		"cluster", cluster,
		"local_db", localDB,
		"local_table", localTable,
	)

	// Recursively get info for the underlying local table.
	underlyingInfo, err := c.GetTableInfo(ctx, localDB, localTable)
	if err != nil {
		// If fetching underlying info fails, log a warning and return the original distributed table info.
		c.logger.Warn("failed to get underlying table info for distributed table",
			"error", err,
			"cluster", cluster,
			"local_db", localDB,
			"local_table", localTable,
		)
		return base, nil
	}

	// Construct the final TableInfo, merging distributed table identity
	// with the structure (columns, sort keys) of the underlying local table.
	return &TableInfo{
		Database:     base.Database,     // Keep original DB name.
		Name:         base.Name,         // Keep original table name.
		Engine:       base.Engine,       // Keep "Distributed" engine type.
		EngineParams: base.EngineParams, // Keep distributed engine parameters.
		CreateQuery:  base.CreateQuery,  // Keep distributed CREATE statement.
		Columns:      underlyingInfo.Columns,
		ExtColumns:   underlyingInfo.ExtColumns,
		SortKeys:     underlyingInfo.SortKeys,
	}, nil
}

// parseEngineParams attempts to parse the parameters from a full engine string
// (e.g., "MergeTree() PARTITION BY toYYYYMM(date) ORDER BY (date, id)").
// This function extracts only the engine constructor parameters, not the full engine clause.
func parseEngineParams(engineFull string) []string {
	// Find the engine constructor parentheses (the first pair)
	start := strings.Index(engineFull, "(")
	if start == -1 {
		return nil // No parameters found
	}

	// Find the matching closing parenthesis by counting nested levels
	parenCount := 0
	end := -1
	for i := start; i < len(engineFull); i++ {
		if engineFull[i] == '(' {
			parenCount++
		} else if engineFull[i] == ')' {
			parenCount--
			if parenCount == 0 {
				end = i
				break
			}
		}
	}

	if end == -1 || start >= end {
		return nil // No matching closing parenthesis found
	}

	paramsStr := engineFull[start+1 : end]

	params := make([]string, 0)
	var currentParam strings.Builder
	inQuote := false
	nestedLevel := 0 // Handle nested parentheses within parameters.

	for _, char := range paramsStr {
		switch char {
		case '\'':
			inQuote = !inQuote
			currentParam.WriteRune(char)
		case '(':
			if !inQuote {
				nestedLevel++
			}
			currentParam.WriteRune(char)
		case ')':
			if !inQuote {
				nestedLevel--
			}
			currentParam.WriteRune(char)
		case ',':
			// Split only if not inside quotes and not inside nested parentheses.
			if !inQuote && nestedLevel == 0 {
				param := strings.TrimSpace(currentParam.String())
				// Optionally remove surrounding quotes from the parameter itself.
				if len(param) >= 2 && param[0] == '\'' && param[len(param)-1] == '\'' {
					param = param[1 : len(param)-1]
				}
				params = append(params, param)
				currentParam.Reset() // Start next parameter.
			} else {
				currentParam.WriteRune(char)
			}
		default:
			currentParam.WriteRune(char)
		}
	}

	// Add the last parameter accumulated.
	if currentParam.Len() > 0 {
		param := strings.TrimSpace(currentParam.String())
		if len(param) >= 2 && param[0] == '\'' && param[len(param)-1] == '\'' {
			param = param[1 : len(param)-1]
		}
		params = append(params, param)
	}

	return params
}

// parseSortKeys attempts to extract individual column names from the sorting_key string.
// It handles simple cases and tuple() but might fail on complex expressions.
func parseSortKeys(sortingKey string) []string {
	if sortingKey == "" {
		return nil
	}

	// Basic handling: remove tuple() if present.
	trimmedKey := strings.TrimSpace(sortingKey)
	if strings.HasPrefix(trimmedKey, "tuple(") && strings.HasSuffix(trimmedKey, ")") {
		trimmedKey = trimmedKey[6 : len(trimmedKey)-1]
	} else if strings.HasPrefix(trimmedKey, "(") && strings.HasSuffix(trimmedKey, ")") {
		// Handle cases like ORDER BY (col1, col2)
		trimmedKey = trimmedKey[1 : len(trimmedKey)-1]
	}

	// Split by comma, then trim spaces and quotes.
	// This won't handle commas inside function calls correctly.
	rawKeys := strings.Split(trimmedKey, ",")
	keys := make([]string, 0, len(rawKeys))
	for _, key := range rawKeys {
		trimmed := strings.TrimSpace(key)
		// Further strip potential backticks or quotes if needed, though identifiers
		// usually don't contain them after parsing functions like tuple().
		// Basic identifier extraction:
		re := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*`) // Simple identifier regex
		match := re.FindString(trimmed)
		if match != "" && !isKeyword(match) { // Check if it's not a keyword
			keys = append(keys, match)
		}
	}

	return keys
}

// isKeyword checks if a string is a common ClickHouse keyword
// to avoid misinterpreting them as column names in sort keys.
func isKeyword(s string) bool {
	// Case-insensitive check.
	lowerS := strings.ToLower(s)
	// Add more keywords if needed based on common sort key expressions.
	keywords := map[string]bool{
		"tuple": true, "array": true, "map": true,
		"as": true, "by": true, "in": true, "is": true,
		"not": true, "null": true, "or": true, "and": true,
		// Potentially date/time functions if used without args:
		// "now": true, "today": true,
	}
	return keywords[lowerS]
}

// Ping checks the connectivity to the ClickHouse server and optionally verifies a table exists.
// It uses short timeouts internally. Returns nil on success, or an error indicating the failure reason.
func (c *Client) Ping(ctx context.Context, database string, table string) error {
	if c.conn == nil {
		if c.metrics != nil {
			c.metrics.RecordConnectionValidation(false)
		}
		return errors.New("clickhouse connection is nil")
	}

	// 1. Check basic connection with a short timeout.
	pingCtx, pingCancel := context.WithTimeout(ctx, 1*time.Second)
	defer pingCancel()

	if err := c.conn.Ping(pingCtx); err != nil {
		if c.metrics != nil {
			c.metrics.RecordConnectionValidation(false)
			c.metrics.UpdateConnectionStatus(false)
		}

		// Check if the error is due to the context deadline exceeding
		if errors.Is(err, context.DeadlineExceeded) {
			c.logger.Debug("ping timed out after 1 second")
			return fmt.Errorf("ping timed out: %w", err)
		}
		return fmt.Errorf("ping failed: %w", err)
	}

	// 2. If database and table are provided, check table existence.
	if database == "" || table == "" {
		if c.metrics != nil {
			c.metrics.RecordConnectionValidation(true)
			c.metrics.UpdateConnectionStatus(true)
		}
		return nil // Basic ping successful, no table check needed.
	}

	tableCtx, tableCancel := context.WithTimeout(ctx, 1*time.Second)
	defer tableCancel()

	// Query system.tables to check if the table exists. Using QueryRow and Scan.
	// If the table doesn't exist, QueryRow will return an error (sql.ErrNoRows or similar).
	query := `SELECT 1 FROM system.tables WHERE database = ? AND name = ? LIMIT 1`
	// Use uint8 as the target type for scanning SELECT 1, as recommended by the driver error.
	var exists uint8

	// No need for executeQueryWithHooks here, it's a simple metadata check.
	err := c.conn.QueryRow(tableCtx, query, database, table).Scan(&exists)
	if err != nil {
		if c.metrics != nil {
			c.metrics.RecordConnectionValidation(false)
			c.metrics.UpdateConnectionStatus(false)
		}

		if errors.Is(err, context.DeadlineExceeded) {
			c.logger.Debug("table check timed out", "database", database, "table", table, "timeout", "1s")
			return fmt.Errorf("table check timed out for %s.%s: %w", database, table, err)
		}
		// Check specifically for sql.ErrNoRows which indicates the table doesn't exist.
		// The clickhouse-go driver might wrap this, so checking the string might be necessary
		// if errors.Is(err, sql.ErrNoRows) doesn't work reliably across versions.
		// For now, we rely on the error message in the log.
		if strings.Contains(err.Error(), "no rows in result set") {
			c.logger.Debug("table not found in system.tables", "database", database, "table", table)
			return fmt.Errorf("table '%s.%s' not found: %w", database, table, err)
		} else {
			// Log other scan/query errors.
			c.logger.Debug("table existence check query failed", "database", database, "table", table, "error", err)
			return fmt.Errorf("checking table '%s.%s' failed: %w", database, table, err)
		}
	}

	// If Scan succeeds without error, the table exists.
	if c.metrics != nil {
		c.metrics.RecordConnectionValidation(true)
		c.metrics.UpdateConnectionStatus(true)
	}
	return nil
}
