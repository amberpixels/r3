// Package enginesql provides a shared base driver for all r3 CRUD implementations
// backed by database/sql. It contains the Flavor configuration, reflection-based
// struct metadata, SQL building helpers, and BaseCRUD / BaseRaw generic types.
//
// Driver-specific packages (pgx, pq, sqlite3, mysql, etc.) embed BaseCRUD and
// only override behavior that differs (e.g. MySQL's Create without RETURNING).
//
// This package is public so that third-party drivers can reuse it.
package enginesql
