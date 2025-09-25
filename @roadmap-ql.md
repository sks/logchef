# LogChefQL Roadmap (frontend/src/utils/logchefql)

## Scope
- Improve safety, correctness, ergonomics, and extensibility of LogChefQL for log exploration.
- Keep diffs small, avoid new deps, and align with existing TS/Vue patterns.

## Priorities
- P0 = safety/correctness fixes
- P1 = UX and developer experience
- P2 = maintainability/extensibility

## P0 – Safety & correctness
- SQL string escaping in path parameters (JSON/Map access)
  - Risk: user-provided path segments are injected into SQL quotes without escaping backslashes/single quotes.
  - Files: `frontend/src/utils/logchefql/sql-generator.ts` (generateJsonExtraction, generateMapAccess, select aliasing)
  - Action: sanitize path segments (strip quotes) then escape `'` and `\` inside SQL string literals.
  - Example helper:
    ```ts
    // Escape for SQL single-quoted string literals
    const esc = (s: string) => s.replace(/['\\]/g, m => (m === '\\' ? '\\\\' : "''"));
    // Usage
    const pathParams = path.map(seg => `'${esc(seg.replace(/["']/g, ''))}'`).join(', ');
    ```

- Boolean keyword tokenization mis-parses
  - Risk: words like "order" or "android" partially match and become `bool` tokens.
  - File: `frontend/src/utils/logchefql/tokenizer.ts`
  - Action: match whole words for `and|or`.
  - Example:
    ```ts
    if (/[a-zA-Z]/.test(char)) {
      const word = input.slice(i).match(/^[a-zA-Z]+/)?.[0] ?? '';
      const lower = word.toLowerCase();
      if (lower === 'and' || lower === 'or') { /* emit bool token */ }
    }
    ```

- Unterminated string literal should error
  - Risk: currently can silently succeed without a clear diagnostic.
  - File: `frontend/src/utils/logchefql/tokenizer.ts`
  - Action: after loop, if `inString` is true, push `UNTERMINATED_STRING` with position.

- Identifier quoting
  - Note: keep backticks for now (decision). Consider double quotes later if needed for strict ClickHouse modes.

### P0 acceptance criteria
- Queries with JSON/Map paths containing `'` or `\` do not break SQL and return correct results.
- Words like "order" are not tokenized as boolean ops; `and`/`or` recognized case-insensitively as whole words.
- Unterminated strings produce a structured error with line/column.
- No change in output for existing valid queries.

## P1 – UX and correctness
- Structured, position-aware parser errors
  - Files: `parser.ts`, `errors.ts`
  - Action: replace generic throws with `errors.push({ code, message, position })`; standardize codes to map to friendly UI texts.

- Autocomplete/context hints for editors
  - File: `parser.ts`, `types.ts`
  - Action: maintain a lightweight `ParseContext` (expected next tokens) and expose via `getContext()`.

- Preserve quotedness for values
  - File: `parser.ts`, `sql-generator.ts`
  - Action: if a value was quoted, keep it as a string even if numeric-looking; only coerce when unquoted.

- Cache ergonomics during exploration
  - File: `cache.ts`
  - Action: refresh timestamp on hit (LRU-ish); allow `schemaVersion` in key to invalidate across grammar/SQL changes.

### P1 acceptance criteria
- UI can render precise error messages and suggestions with exact positions.
- Editor can suggest `and|or`, operators, or values based on `ParseContext`.
- Quoted numeric strings behave as strings; unquoted numerics behave as numbers.
- Cache hit refreshes TTL; bumping `schemaVersion` invalidates stale entries.

## P2 – Maintainability & extensibility
- Tokenizer state machine
  - File: `tokenizer.ts`
  - Action: replace ad-hoc if/regex chain with a small DFA (`switch(state)`) for clarity and performance on long inputs.

- AST for time ranges and future functions
  - File: `types.ts`
  - Action: add nodes like `{ type: 'range', from: Value, to: Value }` and pave path for `now()`, `1h` literals.

- Dialect visitor interface
  - File: `sql-generator.ts`
  - Action: separate `SQLVisitor` interface from `ClickHouseVisitor` to enable future dialects.

- Vue 3 integration helper
  - File: `api.ts`, `index.ts`
  - Action: export `useLogchefQL()` composable to provide tokens, errors, and parse context; keep parsing in a worker if needed.

### P2 acceptance criteria
- Tokenizer easier to extend and reason about; CI perf stable on long queries.
- Range queries representable in AST; no changes to existing syntax until enabled.
- SQL generation pluggable by dialect without touching parser or AST.
- Simpler integration for the editor; component stays thin.

## Tests to add
- Escaped quotes and backslashes in nested fields/path segments.
- Unterminated string literal.
- Boolean keyword boundaries (e.g., "order" not treated as `or`).
- Quoted vs unquoted numeric comparisons.
- Cache invalidation via `schemaVersion` and TTL refresh on hits.

## Migration/rollout plan
- Phase 1 (P0): implement and ship behind unit tests; verify generated SQL for representative queries.
- Phase 2 (P1): surface structured errors/context to UI; keep feature flags for editor hints if needed.
- Phase 3 (P2): refactor tokenizer incrementally; introduce AST nodes and visitor interface without changing current behavior.

## Decisions
- Keep backticks for identifier quoting for now; revisit if ClickHouse settings change.

## References
- Code: `frontend/src/utils/logchefql/` (tokenizer, parser, sql-generator, errors, cache, types, api, index)
