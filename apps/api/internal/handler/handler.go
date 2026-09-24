package handler

import (
	"context"
	"log/slog"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// APIStore is the combined read/write interface required by the HTTP handlers.
type APIStore interface {
	store.Store
	store.QueryStore
	store.LiveStore
	store.ArchiveStore
	store.WatchdogStore
	store.ContractUpgradeStore
	store.HealthScoreStore
	store.APIKeyStore
	store.AlertSubscriptionStore
	store.WatchlistStore
	store.UserStore
	store.PerformanceStore
}

// Pinger is implemented by both the postgres pool and the Redis client.
type Pinger interface {
	Ping(ctx context.Context) error
}

type RedisClient interface {
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) (bool, error)
}

// ColdEventReader serves events that have been archived out of Postgres into
// cold storage (issue #146). It is satisfied by *coldstorage.Reader. A nil
// Cold disables the fallback, which is the default for local development and
// for deployments that have not configured a cold bucket.
type ColdEventReader interface {
	Events(ctx context.Context, contractID string, from, to uint32, limit int) ([]store.Event, error)
}

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	Store       APIStore
	DB          Pinger
	Redis       Pinger
	RedisClient RedisClient
	Logger      *slog.Logger
	// Cold is optional; when set, event queries fall back to object storage for
	// ledger ranges that are no longer in Postgres.
	Cold ColdEventReader
}
