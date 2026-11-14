package store

import "database/sql"

// This file is wacli-reader's only addition to an otherwise byte-identical
// copy of upstream's internal/store. It is a new file rather than an edit so
// that rebases never conflict on it.
//
// The reader never runs migrations: it inherits whatever schema the upstream
// writer last applied, while its queries are upstream's current ones. When the
// writer is older than the reader, queries fail with "no such column" or "no
// such table". These two accessors let the reader say why.

// SchemaVersion reports the highest migration version recorded in the store,
// or 0 when the store has never been migrated.
func (d *DB) SchemaVersion() (int, error) {
	exists, err := d.tableExists("schema_migrations")
	if err != nil || !exists {
		return 0, err
	}
	var v sql.NullInt64
	if err := d.sql.QueryRow(`SELECT max(version) FROM schema_migrations`).Scan(&v); err != nil {
		return 0, err
	}
	return int(v.Int64), nil
}

// LatestSchemaVersion is the schema version this build's queries are written
// against: the newest migration upstream's writer would apply.
func LatestSchemaVersion() int {
	latest := 0
	for _, m := range schemaMigrations {
		latest = max(latest, m.version)
	}
	return latest
}
