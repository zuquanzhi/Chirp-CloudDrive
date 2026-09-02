package sqlite

import (
	"database/sql"

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
		gender TEXT
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

	if _, err := db.Exec(createUsers); err != nil {
		return nil, err
	}
	if _, err := db.Exec(createResources); err != nil {
		return nil, err
	}
	if _, err := db.Exec(createNotifications); err != nil {
		return nil, err
	}

	return db, nil
}
