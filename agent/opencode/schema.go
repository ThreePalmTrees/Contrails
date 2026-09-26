package opencode

import (
	"database/sql"
	"errors"
	"fmt"
)

// Session table names, newest first. Released OpenCode 2.0.x builds keep
// sessions in session_v2; the OpenCode source (HEAD) names the table
// session. Both carry every column this package reads.
var sessionTableNames = []string{"session", "session_v2"}

// errNoSessionTable is returned when the database has neither session table.
var errNoSessionTable = errors.New("no OpenCode session table (session or session_v2) found")

// sessionTable returns the name of the table that holds session rows.
//
// The table session_message points at through its foreign key is used when
// it is one of the known names: a database could hold both tables (e.g. a
// leftover from another schema line), and the foreign key says which one
// the messages belong to. Otherwise the first known table that exists wins,
// in the order of sessionTableNames.
//
// The returned name is always one of sessionTableNames, so it is safe to
// format into a query.
func sessionTable(db *sql.DB) (string, error) {
	existing, err := existingTables(db)
	if err != nil {
		return "", err
	}

	if referenced, err := messageSessionTable(db); err == nil {
		for _, name := range sessionTableNames {
			if name == referenced && existing[name] {
				return name, nil
			}
		}
	}

	for _, name := range sessionTableNames {
		if existing[name] {
			return name, nil
		}
	}
	return "", errNoSessionTable
}

// existingTables reports which of the known session table names exist.
func existingTables(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master
		WHERE type = 'table' AND name IN ('session', 'session_v2')`)
	if err != nil {
		return nil, fmt.Errorf("listing tables: %w", err)
	}
	defer rows.Close()

	existing := make(map[string]bool, len(sessionTableNames))
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning table name: %w", err)
		}
		existing[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating tables: %w", err)
	}
	return existing, nil
}

// messageSessionTable returns the table session_message.session_id refers
// to, or "" when the column has no foreign key.
func messageSessionTable(db *sql.DB) (string, error) {
	rows, err := db.Query("PRAGMA foreign_key_list(session_message)")
	if err != nil {
		return "", fmt.Errorf("reading session_message foreign keys: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("reading foreign key columns: %w", err)
	}

	var referenced string
	for rows.Next() {
		// Scan by column name: the pragma's column set is fixed, but
		// reading it by name keeps this independent of the order.
		values := make([]sql.NullString, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return "", fmt.Errorf("scanning foreign key: %w", err)
		}
		fields := make(map[string]string, len(columns))
		for i, column := range columns {
			fields[column] = values[i].String
		}
		if fields["from"] == "session_id" {
			referenced = fields["table"]
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterating foreign keys: %w", err)
	}
	return referenced, nil
}
