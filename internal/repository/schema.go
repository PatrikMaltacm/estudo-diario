package repository

import (
	"context"
	"database/sql"
	"fmt"
)

// EnsureSchema executa migrações idempotentes no banco de dados para garantir que
// as tabelas e colunas necessárias para o sistema e para o multi-idioma existam.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id VARCHAR(36) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			password VARCHAR(255) NOT NULL,
			points INT NOT NULL DEFAULT 0,
			preferred_language VARCHAR(10) NOT NULL DEFAULT 'pt-BR'
		);`,
		`CREATE TABLE IF NOT EXISTS study_sessions (
			id BIGSERIAL PRIMARY KEY,
			user_id VARCHAR(36) NOT NULL REFERENCES users(id),
			study_date VARCHAR(10) NOT NULL,
			topic VARCHAR(500) NOT NULL,
			started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			submitted_at TIMESTAMPTZ,
			summary TEXT,
			evaluation JSONB,
			language VARCHAR(10) NOT NULL DEFAULT 'pt-BR',
			UNIQUE(user_id, study_date)
		);`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS preferred_language VARCHAR(10) NOT NULL DEFAULT 'pt-BR';`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS email_reminders_enabled BOOLEAN NOT NULL DEFAULT TRUE;`,
		`ALTER TABLE study_sessions ADD COLUMN IF NOT EXISTS language VARCHAR(10) NOT NULL DEFAULT 'pt-BR';`,
	}

	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("falha ao executar migração de schema (%s): %w", q, err)
		}
	}
	return nil
}
