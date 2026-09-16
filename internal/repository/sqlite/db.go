package sqlite

import (
	"database/sql"
	"strings"

	_ "modernc.org/sqlite"
)

func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	createUsers := `CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		email TEXT UNIQUE,
		password TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		phone_number TEXT,
		school TEXT,
		student_id TEXT,
		birthdate TEXT,
		address TEXT,
		gender TEXT,
		role TEXT DEFAULT 'USER',
		quota INTEGER DEFAULT 1073741824,
		used INTEGER DEFAULT 0
	);`

	createResources := `CREATE TABLE IF NOT EXISTS resources (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		owner_id INTEGER,
		title TEXT,
		description TEXT,
		filename TEXT,
		original_name TEXT,
		size INTEGER,
		file_hash TEXT,
		status TEXT DEFAULT 'PENDING',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		subject TEXT,
		type TEXT,
		FOREIGN KEY(owner_id) REFERENCES users(id)
	);`

	createNotifications := `CREATE TABLE IF NOT EXISTS notifications (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		content TEXT,
		is_read BOOLEAN DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);`

	createFolders := `CREATE TABLE IF NOT EXISTS folders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		owner_id INTEGER NOT NULL,
		parent_id INTEGER,
		name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		deleted_at DATETIME,
		FOREIGN KEY(owner_id) REFERENCES users(id),
		FOREIGN KEY(parent_id) REFERENCES folders(id)
	);`

	createShares := `CREATE TABLE IF NOT EXISTS shares (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		token TEXT UNIQUE NOT NULL,
		resource_id INTEGER NOT NULL,
		owner_id INTEGER NOT NULL,
		password TEXT,
		expires_at DATETIME,
		downloads INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(resource_id) REFERENCES resources(id),
		FOREIGN KEY(owner_id) REFERENCES users(id)
	);`

	createActivities := `CREATE TABLE IF NOT EXISTS activities (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		action TEXT NOT NULL,
		target_kind TEXT NOT NULL,
		target_id INTEGER,
		target_name TEXT,
		detail TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);`

	createUploadSessions := `CREATE TABLE IF NOT EXISTS upload_sessions (
		id TEXT PRIMARY KEY,
		owner_id INTEGER NOT NULL,
		folder_id INTEGER,
		filename TEXT NOT NULL,
		size INTEGER NOT NULL,
		chunk_size INTEGER NOT NULL,
		total_chunks INTEGER NOT NULL,
		file_hash TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(owner_id) REFERENCES users(id)
	);`

	if _, err := db.Exec(createUsers); err != nil {
		return nil, err
	}
	if _, err := db.Exec(createResources); err != nil {
		return nil, err
	}
	if _, err := db.Exec(createNotifications); err != nil {
		return nil, err
	}
	if _, err := db.Exec(createFolders); err != nil {
		return nil, err
	}
	if _, err := db.Exec(createShares); err != nil {
		return nil, err
	}
	if _, err := db.Exec(createActivities); err != nil {
		return nil, err
	}
	if _, err := db.Exec(createUploadSessions); err != nil {
		return nil, err
	}

	// Migrations for databases created by older versions (idempotent).
	migrations := []string{
		`ALTER TABLE users ADD COLUMN role TEXT DEFAULT 'USER'`,
		`ALTER TABLE users ADD COLUMN quota INTEGER DEFAULT 1073741824`,
		`ALTER TABLE users ADD COLUMN used INTEGER DEFAULT 0`,
		`ALTER TABLE resources ADD COLUMN folder_id INTEGER`,
		`ALTER TABLE resources ADD COLUMN deleted_at DATETIME`,
		`ALTER TABLE resources ADD COLUMN version_group TEXT DEFAULT ''`,
		`ALTER TABLE resources ADD COLUMN version INTEGER DEFAULT 1`,
		`ALTER TABLE resources ADD COLUMN is_latest BOOLEAN DEFAULT 1`,
	}
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return nil, err
		}
	}

	return db, nil
}
