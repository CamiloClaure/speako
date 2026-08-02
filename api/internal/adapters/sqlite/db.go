package sqlite

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

// Open opens the SQLite database at path and applies the connection pragmas.
// The _pragma DSN parameters run on every pooled connection, because pragmas
// are per-connection; the explicit Exec below re-applies them to the first
// connection so a failure is surfaced here rather than on a later query.
func Open(path string) (*sql.DB, error) {
	const pragmas = "_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", path+"?"+pragmas)
	if err != nil {
		return nil, err
	}
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}
	return db, nil
}
