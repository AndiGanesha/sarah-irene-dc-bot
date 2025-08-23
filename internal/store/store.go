package store

import (
	"context"
	"database/sql"
	_ "modernc.org/sqlite"
)

type DB struct{ *sql.DB }

func Open(dsn string) (*DB, func() error, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil { return nil, nil, err }
	if err := migrate(db); err != nil { return nil, nil, err }
	return &DB{db}, db.Close, nil
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`PRAGMA journal_mode=WAL;`,
		`PRAGMA synchronous=NORMAL;`,
		`CREATE TABLE IF NOT EXISTS settings (id INTEGER PRIMARY KEY CHECK(id=1), guild_id TEXT NOT NULL, voice_channel_id TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS subscribers (user_id TEXT PRIMARY KEY);`,
		`CREATE TABLE IF NOT EXISTS vc_presence (user_id TEXT PRIMARY KEY, game TEXT, updated_at INTEGER NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS last_dm (user_id TEXT PRIMARY KEY, state_hash TEXT NOT NULL, sent_at INTEGER NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS dm_outbox (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id TEXT NOT NULL, reason TEXT NOT NULL, body TEXT NOT NULL, available_at INTEGER NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, picked_at INTEGER);`,
		`CREATE INDEX IF NOT EXISTS idx_dm_outbox_available ON dm_outbox(available_at);`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil { return err }
	}
	return nil
}

// TODO: add typed CRUD methods in next step
