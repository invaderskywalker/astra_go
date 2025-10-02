package psql

import (
	"astra/astra/config"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	Pool *pgxpool.Pool
}

func NewDatabase(ctx context.Context, cfg config.Config) (*Database, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	return &Database{Pool: pool}, nil
}

func (db *Database) Close() {
	db.Pool.Close()
}
