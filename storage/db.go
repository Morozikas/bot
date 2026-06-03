// Пакет storage — хранение данных.
// db.go — PostgreSQL: создание схемы, сохранение сигналов и результатов торговли.
package storage

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// DB — обёртка над *sql.DB с вспомогательными методами.
type DB struct {
	conn *sql.DB
}

// NewDB открывает соединение с PostgreSQL.
func NewDB(dsn string) (*DB, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("открытие БД: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping БД: %w", err)
	}
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	return &DB{conn: conn}, nil
}

// Close закрывает соединение.
func (d *DB) Close() error { return d.conn.Close() }

// Migrate создаёт таблицы при первом запуске.
func (d *DB) Migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS signals (
			id          TEXT PRIMARY KEY,
			symbol      TEXT NOT NULL,
			direction   TEXT NOT NULL,
			entry       DOUBLE PRECISION,
			stop_loss   DOUBLE PRECISION,
			take_profit DOUBLE PRECISION,
			rr          DOUBLE PRECISION,
			score       DOUBLE PRECISION,
			grade       TEXT,
			probability DOUBLE PRECISION,
			setup       TEXT,
			created_at  TIMESTAMPTZ DEFAULT NOW(),
			published   BOOLEAN DEFAULT FALSE,
			executed    BOOLEAN DEFAULT FALSE
		)`,
		`CREATE TABLE IF NOT EXISTS signal_outcomes (
			signal_id  TEXT PRIMARY KEY REFERENCES signals(id),
			outcome    TEXT,
			pnl_pct    DOUBLE PRECISION,
			closed_at  TIMESTAMPTZ
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id           TEXT PRIMARY KEY,
			name         TEXT,
			api_key      TEXT,
			api_secret   TEXT,
			testnet      BOOLEAN DEFAULT FALSE,
			risk_pct     DOUBLE PRECISION DEFAULT 1.0,
			max_leverage INT DEFAULT 10,
			active       BOOLEAN DEFAULT TRUE,
			created_at   TIMESTAMPTZ DEFAULT NOW()
		)`,
	}
	for _, q := range queries {
		if _, err := d.conn.Exec(q); err != nil {
			return fmt.Errorf("миграция: %w\nSQL: %s", err, q)
		}
	}
	return nil
}

// SaveSignal сохраняет сигнал в базу данных.
func (d *DB) SaveSignal(id, symbol, direction string, entry, sl, tp, rr, score, prob float64, grade, setup string) error {
	_, err := d.conn.Exec(`
		INSERT INTO signals (id, symbol, direction, entry, stop_loss, take_profit, rr, score, grade, probability, setup)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (id) DO NOTHING`,
		id, symbol, direction, entry, sl, tp, rr, score, grade, prob, setup,
	)
	return err
}

// UpdateOutcome записывает результат сделки.
func (d *DB) UpdateOutcome(signalID, outcome string, pnlPct float64) error {
	_, err := d.conn.Exec(`
		INSERT INTO signal_outcomes (signal_id, outcome, pnl_pct, closed_at)
		VALUES ($1,$2,$3,NOW())
		ON CONFLICT (signal_id) DO UPDATE SET outcome=$2, pnl_pct=$3, closed_at=NOW()`,
		signalID, outcome, pnlPct,
	)
	return err
}
