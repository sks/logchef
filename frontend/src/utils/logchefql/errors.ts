import type { ParseError } from './types';

export interface UserFriendlyError {
  code: string;
  message: string;
  suggestion?: string;
  position?: { line: number; column: number };
}

export class LogChefQLErrorHandler {
  static getUserFriendlyError(error: ParseError, context?: string): UserFriendlyError {
    const errorMap: Record<string, UserFriendlyError> = {
      'UNTERMINATED_STRING': {
        code: 'UNTERMINATED_STRING',
        message: 'Unterminated string literal',
        suggestion: 'Close the string with a matching quote character'
      },
      'UNEXPECTED_END': {
        code: 'UNEXPECTED_END',
        message: 'Unexpected end of input',
        suggestion: 'Complete your query - it appears to be cut off'
      },
      'UNEXPECTED_TOKEN': {
        code: 'UNEXPECTED_TOKEN',
        message: context ? `Unexpected token: "${context}"` : 'Unexpected token',
        suggestion: 'Check for typos and valid LogChefQL syntax'
      },
      'EXPECTED_OPERATOR': {
        code: 'EXPECTED_OPERATOR',
        message: 'Expected an operator after field name',
        suggestion: 'Add an operator like =, !=, >, <, >=, <=, ~, !~'
      },
      'EXPECTED_VALUE': {
        code: 'EXPECTED_VALUE',
        message: 'Expected a value after operator',
        suggestion: 'Add a value after the operator (e.g., field = "value")'
      },
      'EXPECTED_CLOSING_PAREN': {
        code: 'EXPECTED_CLOSING_PAREN',
        message: 'Expected closing parenthesis',
        suggestion: 'Add a closing parenthesis ) to match the opening one'
      },
      'UNKNOWN_OPERATOR': {
        code: 'UNKNOWN_OPERATOR',
        message: context ? `Unknown operator: "${context}"` : 'Unknown operator',
        suggestion: 'Use a valid operator: =, !=, >, <, >=, <=, ~, !~'
      },
      'UNKNOWN_BOOLEAN_OPERATOR': {
        code: 'UNKNOWN_BOOLEAN_OPERATOR',
        message: context ? `Unknown boolean operator: "${context}"` : 'Unknown boolean operator',
        suggestion: 'Use "and" or "or" to combine conditions'
      },
      'INVALID_TOKEN_TYPE': {
        code: 'INVALID_TOKEN_TYPE',
        message: context ? `Invalid token type: "${context}"` : 'Invalid token type',
        suggestion: 'Check your query syntax'
      },
    };

    return errorMap[error.code] || {
      code: 'UNKNOWN_ERROR',
      message: error.message,
      suggestion: 'Check your query syntax'
    };
  }

  static createErrorFromMessage(message: string, position?: { line: number; column: number }): UserFriendlyError {
    return {
      code: 'CUSTOM_ERROR',
      message,
      position,
      suggestion: 'Review your query and try again'
    };
  }

  static createParseError(code: string, message?: string, position?: { line: number; column: number }): ParseError {
    return {
      code,
      message: message || code,
      position
    };
  }
}

// Helper functions to create specific errors with position info
export function createUnexpectedEndError(position?: { line: number; column: number }): ParseError {
  return LogChefQLErrorHandler.createParseError('UNEXPECTED_END', 'Unexpected end of input', position);
}

export function createUnexpectedTokenError(tokenValue: string, position?: { line: number; column: number }): ParseError {
  return LogChefQLErrorHandler.createParseError('UNEXPECTED_TOKEN', `Unexpected token: ${tokenValue}`, position);
}

export function createExpectedOperatorError(position?: { line: number; column: number }): ParseError {
  return LogChefQLErrorHandler.createParseError('EXPECTED_OPERATOR', 'Expected an operator after field name', position);
}

export function createExpectedValueError(position?: { line: number; column: number }): ParseError {
  return LogChefQLErrorHandler.createParseError('EXPECTED_VALUE', 'Expected a value after operator', position);
}

export function createExpectedClosingParenError(position?: { line: number; column: number }): ParseError {
  return LogChefQLErrorHandler.createParseError('EXPECTED_CLOSING_PAREN', 'Expected closing parenthesis', position);
}

export function createUnknownOperatorError(operator: string, position?: { line: number; column: number }): ParseError {
  return LogChefQLErrorHandler.createParseError('UNKNOWN_OPERATOR', `Unknown operator: ${operator}`, position);
}

export function createUnknownBooleanOperatorError(operator: string, position?: { line: number; column: number }): ParseError {
  return LogChefQLErrorHandler.createParseError('UNKNOWN_BOOLEAN_OPERATOR', `Unknown boolean operator: ${operator}`, position);
}

export function createInvalidTokenTypeError(tokenType: string, position?: { line: number; column: number }): ParseError {
  return LogChefQLErrorHandler.createParseError('INVALID_TOKEN_TYPE', `Unexpected token type: ${tokenType}`, position);
}