export enum Operator {
  EQUALS = '=',
  NOT_EQUALS = '!=',
  REGEX = '~',
  NOT_REGEX = '!~',
  GT = '>',
  LT = '<',
  GTE = '>=',
  LTE = '<=',
}

export enum BoolOperator {
  AND = 'AND',
  OR = 'OR',
}

export type Value = string | number | boolean | null;

export type ASTNode =
  | {
      type: 'expression';
      key: string | NestedField;
      operator: Operator;
      value: Value;
    }
  | {
      type: 'logical';
      operator: BoolOperator;
      children: ASTNode[];
    }
  | {
      type: 'group';
      children: ASTNode[];
    }
  | {
      type: 'query';
      where?: ASTNode;
      select?: SelectField[];
    };

export interface SelectField {
  field: string | NestedField;
  alias?: string;
}

export interface NestedField {
  base: string;
  path: string[];
}

export interface Token {
  type: 'key' | 'operator' | 'value' | 'paren' | 'bool' | 'pipe';
  value: string;
  position: {
    line: number;
    column: number;
  };
  // Indicates the token originated from a quoted string literal
  quoted?: boolean;
}

export interface ParseError {
  code: string;
  message: string;
  position?: {
    line: number;
    column: number;
  };
}