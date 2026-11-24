package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// New returns a pgxpool connection pool for the given DSN.
func New(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}

	config.MinConns = 2
	// 50 was oversized for this app's actual concurrency (a ~30-person family
	// league); every connection the pool opens pays Supabase's pooler-side
	// per-connection setup cost (role/auth checks against Postgres), so a
	// smaller ceiling means less of that overhead under any traffic burst.
	config.MaxConns = 10
	// shorter than Postgres default idle timeout to avoid "connection reset" on reuse
	config.MaxConnIdleTime = 5 * time.Minute
	// Was 30m, which recycled every connection — even ones in constant use —
	// twice an hour, repaying the pooler's connection-setup cost each time.
	// That churn alone was ~35% of total Supabase query time (role/auth
	// checks), dwarfing actual app queries. Idle connections still get
	// reaped by MaxConnIdleTime above; this only stops recycling busy ones.
	config.MaxConnLifetime = 2 * time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect to db: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return pool, nil
}
