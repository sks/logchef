import type { ASTNode, NestedField } from './types';
import { Operator } from './types';

export interface ColumnInfo {
  name: string;
  type: string;
}

export interface SchemaInfo {
  columns: ColumnInfo[];
}

export class SQLVisitor {
  private readonly parameterized: boolean;
  private params: unknown[] = [];
  private schema?: SchemaInfo;

  constructor(parameterized = false, schema?: SchemaInfo) {
    this.parameterized = parameterized;
    this.schema = schema;
  }

  public generate(node: ASTNode): { sql: string; params: unknown[] } {
    this.params = [];
    return {
      sql: this.visit(node),
      params: this.parameterized ? this.params : [],
    };
  }

  private visit(node: ASTNode): string {
    switch (node.type) {
      case 'expression':
        return this.visitExpression(node);
      case 'logical':
        return this.visitLogical(node);
      case 'group':
        return this.visitGroup(node);
      default:
        return '';
    }
  }

  private visitExpression(node: ASTNode & { type: 'expression' }): string {
    // Check if we have a nested field
    if (typeof node.key === 'object' && 'base' in node.key) {
      const nestedField = node.key as NestedField;
      const columnType = this.getColumnType(nestedField.base);


      return this.generateNestedFieldAccess(
        nestedField.base,
        nestedField.path,
        columnType,
        node.operator,
        node.value
      );
    }

    // Handle simple field access (original logic)
    const column = this.escapeIdentifier(node.key as string);

    // For regex operations, always handle the value as a string
    // regardless of its original type
    const value = node.operator === Operator.REGEX || node.operator === Operator.NOT_REGEX
      ? this.formatStringValue(node.value)
      : this.formatValue(node.value);

    switch (node.operator) {
      case Operator.REGEX:
        return `positionCaseInsensitive(${column}, ${value}) > 0`;
      case Operator.NOT_REGEX:
        return `positionCaseInsensitive(${column}, ${value}) = 0`;
      case Operator.EQUALS:
        return `${column} = ${value}`;
      case Operator.NOT_EQUALS:
        return `${column} != ${value}`;
      case Operator.GT:
        return `${column} > ${value}`;
      case Operator.LT:
        return `${column} < ${value}`;
      case Operator.GTE:
        return `${column} >= ${value}`;
      case Operator.LTE:
        return `${column} <= ${value}`;
      default:
        return '';
    }
  }

  private visitLogical(node: ASTNode & { type: 'logical' }): string {
    if (node.children.length === 0) {
      return '';
    }

    if (node.children.length === 1) {
      return this.visit(node.children[0]);
    }

    const conditions = node.children.map(child => this.visit(child)).filter(Boolean);

    if (conditions.length === 0) {
      return '';
    }

    if (conditions.length === 1) {
      return conditions[0];
    }

    const operator = node.operator;
    return conditions.map(condition => `(${condition})`).join(` ${operator} `);
  }

  private visitGroup(node: ASTNode & { type: 'group' }): string {
    if (node.children.length === 0) {
      return '';
    }

    if (node.children.length === 1) {
      return this.visit(node.children[0]);
    }

    // Handle multiple expressions in a group - if there's no explicit operator,
    // default to AND (similar to SQL WHERE clause behavior)
    const conditions = node.children.map(child => this.visit(child)).filter(Boolean);

    if (conditions.length === 0) {
      return '';
    }

    if (conditions.length === 1) {
      return conditions[0];
    }

    return `(${conditions.join(' AND ')})`;
  }

  private escapeIdentifier(identifier: string): string {
    return `\`${identifier.replace(/`/g, '``')}\``;
  }

  private formatValue(value: unknown): string {
    if (this.parameterized) {
      this.params.push(value);
      return '?';
    }

    if (value === null) {
      return 'NULL';
    }

    if (typeof value === 'boolean') {
      return value ? '1' : '0';
    }

    if (typeof value === 'number') {
      return value.toString();
    }

    // String value - properly escape using proper SQL escaping (replace single quotes with two single quotes)
    return this.formatStringValue(value);
  }

  // Added helper method to ensure consistent string formatting
  private formatStringValue(value: unknown): string {
    if (this.parameterized) {
      this.params.push(value);
      return '?';
    }

    // Handle null/undefined cases
    if (value === null || value === undefined) {
      return 'NULL';
    }

    // Ensure the value is treated as a string and properly escaped
    // for SQL - use standard SQL escaping (double single quotes)
    const stringValue = String(value);
    const escapedValue = stringValue.replace(/'/g, "''");
    return `'${escapedValue}'`;
  }

  private getColumnType(columnName: string): string | null {
    if (!this.schema) {
      return null;
    }

    const column = this.schema.columns.find(col => col.name === columnName);
    return column?.type ?? null;
  }

  private isMapType(columnType: string): boolean {
    const lowerType = columnType.toLowerCase();
    return lowerType.startsWith('map(');
  }

  private isJsonType(columnType: string): boolean {
    const lowerType = columnType.toLowerCase();
    return lowerType === 'json' || lowerType.startsWith('json(') || lowerType === 'newjson';
  }

  private isStringType(columnType: string): boolean {
    const lowerType = columnType.toLowerCase();
    return lowerType === 'string' ||
           lowerType.startsWith('string(') ||
           lowerType.startsWith('fixedstring(') ||
           lowerType.startsWith('lowcardinality(string)');
  }

  private generateNestedFieldAccess(
    baseColumn: string,
    path: string[],
    columnType: string | null,
    operator: Operator,
    value: unknown
  ): string {
    const formattedValue = operator === Operator.REGEX || operator === Operator.NOT_REGEX
      ? this.formatStringValue(value)
      : this.formatValue(value);

    // If no schema info, fallback to JSON extraction
    if (!columnType) {
      return this.generateJsonExtraction(baseColumn, path, operator, formattedValue);
    }

    // Handle different column types
    if (this.isMapType(columnType)) {
      return this.generateMapAccess(baseColumn, path, operator, formattedValue);
    } else if (this.isJsonType(columnType)) {
      return this.generateJsonExtraction(baseColumn, path, operator, formattedValue);
    } else if (this.isStringType(columnType)) {
      // String column might contain JSON - try JSON extraction
      return this.generateJsonExtraction(baseColumn, path, operator, formattedValue);
    }

    // Fallback to JSON extraction for unknown types
    return this.generateJsonExtraction(baseColumn, path, operator, formattedValue);
  }

  private generateMapAccess(
    baseColumn: string,
    path: string[],
    operator: Operator,
    formattedValue: string
  ): string {
    const escapedColumn = this.escapeIdentifier(baseColumn);

    // For ClickHouse Maps, we can access nested keys using dot notation as a single key
    // e.g., log_attributes['syslog.version'] rather than log_attributes['syslog']['version']
    const fullKey = path.map(segment => segment.replace(/['"]/g, '')).join('.');
    const mapAccess = `${escapedColumn}['${fullKey}']`;

    return this.generateComparisonExpression(mapAccess, operator, formattedValue);
  }

  private generateJsonExtraction(
    baseColumn: string,
    path: string[],
    operator: Operator,
    formattedValue: string
  ): string {
    const escapedColumn = this.escapeIdentifier(baseColumn);
    const jsonPath = path.map(segment => segment.replace(/['"]/g, '')).join('.');

    // Use JSONExtractString for most operations
    const jsonExtract = `JSONExtractString(${escapedColumn}, '${jsonPath}')`;
    return this.generateComparisonExpression(jsonExtract, operator, formattedValue);
  }

  private generateComparisonExpression(
    columnExpression: string,
    operator: Operator,
    formattedValue: string
  ): string {
    switch (operator) {
      case Operator.REGEX:
        return `positionCaseInsensitive(${columnExpression}, ${formattedValue}) > 0`;
      case Operator.NOT_REGEX:
        return `positionCaseInsensitive(${columnExpression}, ${formattedValue}) = 0`;
      case Operator.EQUALS:
        return `${columnExpression} = ${formattedValue}`;
      case Operator.NOT_EQUALS:
        return `${columnExpression} != ${formattedValue}`;
      case Operator.GT:
        return `${columnExpression} > ${formattedValue}`;
      case Operator.LT:
        return `${columnExpression} < ${formattedValue}`;
      case Operator.GTE:
        return `${columnExpression} >= ${formattedValue}`;
      case Operator.LTE:
        return `${columnExpression} <= ${formattedValue}`;
      default:
        return '';
    }
  }
}