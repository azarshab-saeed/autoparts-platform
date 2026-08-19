# Release Notes - Phase 15.14.1

Fixes a PostgreSQL date-range query type inference bug introduced in the Tax / Official Invoice register.

### Fixed
- Tax invoice list no longer fails with `operator does not exist: timestamp with time zone < interval (SQLSTATE 42883)`.
- Sales history and purchase history receive the same preventive fix because they shared the same SQL pattern.
- RC static checks now catch untyped bind-parameter arithmetic with SQL intervals.

No database migration is required.
