package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/middleware"
)

// New builds and returns the HTTP router with all middleware and routes wired.
func New(h *handler.Handler) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(OTelMiddleware)

	r.Use(middleware.RequestID)
	r.Use(middleware.CORS)
	r.Use(middleware.Recoverer(h.Logger))
	r.Use(middleware.Logger(h.Logger))
	r.Use(chiMiddleware.StripSlashes)

	r.Use(middleware.RateLimit(h.RedisClient, h.Store))

	// Health (not rate-limited)
	r.Get("/health", h.Health)
	r.Get("/readyz", h.Readyz)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.ContentTypeJSON)

		// Scoped API key auth. It is applied per route with r.With so chi has
		// already resolved the leaf route pattern when the middleware runs; the
		// required scope is looked up from the metadata table in
		// middleware/scopes.go keyed by that pattern. Requests without a
		// credential still pass on read/write routes (public v0.1 surface),
		// while API key management always requires a credential.
		scope := middleware.RequireScopes(h.Store, h.Logger)
		// RBAC: role enforcement on top of scope. Route wiring uses r.With
		// (same pattern as scope) so the role middleware denies a request
		// that lacks the minimum role regardless of API key scopes.
		contributor := middleware.RequireRole(h.Store, h.Logger, middleware.RoleContributor)
		admin := middleware.RequireRole(h.Store, h.Logger, middleware.RoleAdmin)

		get := func(pattern string, fn http.HandlerFunc) { r.With(scope).Get(pattern, fn) }

		// Stats
		get("/stats/global", h.GlobalStats)

		// Live dashboard feeds (#139): the newest events across all tracked
		// contracts, and per-contract events-per-minute buckets for the
		// sparklines and hot-contracts leaderboard.
		get("/events/recent", h.RecentEvents)
		get("/stats/activity", h.LiveActivity)

		// Contracts. Registration mutates shared state, so it requires at
		// least contributor role. Reads stay open.
		//
		// /contracts/validate is the tracking wizard's read-only pre-flight
		// check (issue #140). It is a POST purely to keep the path
		// unambiguous against /contracts/{id}; it never writes.
		r.With(scope).Post("/contracts/validate", h.ValidateContract)
		r.With(scope, contributor).Post("/contracts", h.RegisterContract)
		get("/contracts", h.ListContracts)
		get("/contracts/{id}", h.GetContract)
		get("/contracts/{id}/events", h.ListEvents)
		get("/contracts/{id}/invocations", h.ListInvocations)
		get("/contracts/{id}/storage", h.ListStorageEntries)
		get("/contracts/{id}/stats", h.ContractStats)
		get("/contracts/{id}/forecast", h.ContractForecast)
		get("/contracts/{id}/snapshot", h.ContractSnapshot)
		get("/contracts/{id}/upgrades", h.ListContractUpgrades)
		get("/contracts/{id}/health-score", h.GetContractHealthScore)
		get("/contracts/{id}/stream", h.StreamEvents)
		get("/contracts/{id}/graph", h.ContractGraph)
		get("/stream/events", h.StreamEventsSSE)


		// API keys (admin scope + admin role).
		r.With(scope, admin).Get("/api-keys", h.ListAPIKeys)
		r.With(scope, admin).Post("/api-keys", h.CreateAPIKey)
		r.With(scope, admin).Delete("/api-keys/{id}", h.RevokeAPIKey)

		// Admin surface. Wrapped by role admin so contributors cannot reach
		// these endpoints even when the API key carries admin scope.
		r.With(admin).Route("/admin", func(r chi.Router) {
			r.Get("/keys", h.ListAPIKeys)
			r.Post("/keys", h.CreateAPIKey)
			r.Delete("/keys/{id}", h.RevokeAPIKey)
		})

		// Watchlist
		r.Route("/watchlist", func(r chi.Router) {
			r.Post("/", h.AddToWatchlist)
			r.Delete("/{contractId}", h.RemoveFromWatchlist)
			r.Get("/", h.ListWatchlist)
			r.Get("/{contractId}/status", h.WatchlistStatus)
		})

		// Watchdog: data from the on-chain sorolens-watchdog contract.
		get("/watchdog/stats", h.WatchdogStats)
		get("/watchdog/alerts", h.ListWatchdogAlerts)
		get("/watchdog/contracts", h.ListMonitoredContracts)
		get("/watchdog/contracts/{id}", h.GetMonitoredContract)
		get("/watchdog/contracts/{id}/health", h.ListHealthChecks)
		get("/watchdog/contracts/{id}/alerts", h.ListWatchdogAlerts)
	})

	// API v2 (issue #144). A parallel namespace carrying the same resources
	// with a consistent envelope (data/pagination), RFC 3339 timestamps, and
	// uniform field names. v1 is untouched; see docs/api-v2.md for the
	// field-by-field mapping.
	//
	// The scope and role middleware are the same as v1, so an API key or role
	// that works against v1 works identically against v2.
	r.Route("/api/v2", func(r chi.Router) {
		r.Use(middleware.ContentTypeJSON)

		scope := middleware.RequireScopes(h.Store, h.Logger)
		contributor := middleware.RequireRole(h.Store, h.Logger, middleware.RoleContributor)
		admin := middleware.RequireRole(h.Store, h.Logger, middleware.RoleAdmin)

		get := func(pattern string, fn http.HandlerFunc) { r.With(scope).Get(pattern, fn) }

		// Live dashboard feeds (#139).
		get("/events/recent", h.V2RecentEvents)
		get("/stats/activity", h.V2LiveActivity)

		// Stats
		get("/stats/global", h.V2GlobalStats)

		// Contracts
		r.With(scope).Post("/contracts/validate", h.V2ValidateContract)
		r.With(scope, contributor).Post("/contracts", h.V2RegisterContract)
		get("/contracts", h.V2ListContracts)
		get("/contracts/{id}", h.V2GetContract)
		get("/contracts/{id}/events", h.V2ListEvents)
		get("/contracts/{id}/invocations", h.V2ListInvocations)
		get("/contracts/{id}/storage", h.V2ListStorageEntries)
		get("/contracts/{id}/stats", h.V2ContractStats)
		get("/contracts/{id}/forecast", h.V2ContractForecast)
		get("/contracts/{id}/snapshot", h.V2ContractSnapshot)
		get("/contracts/{id}/upgrades", h.V2ListContractUpgrades)
		get("/contracts/{id}/health-score", h.V2GetContractHealthScore)
		get("/contracts/{id}/stream", h.V2StreamEvents)
		get("/contracts/{id}/graph", h.V2ContractGraph)

		// API keys (admin scope + admin role). These reuse the v1 handlers and
		// are declared passthrough in docs/api-v2.md.
		r.With(scope, admin).Get("/api-keys", h.ListAPIKeys)
		r.With(scope, admin).Post("/api-keys", h.CreateAPIKey)
		r.With(scope, admin).Delete("/api-keys/{id}", h.RevokeAPIKey)

		// Admin surface, mirroring v1 so v2 has a 1:1 route map.
		r.With(admin).Route("/admin", func(r chi.Router) {
			r.Get("/keys", h.ListAPIKeys)
			r.Post("/keys", h.CreateAPIKey)
			r.Delete("/keys/{id}", h.RevokeAPIKey)
		})

		// Watchlist
		r.Route("/watchlist", func(r chi.Router) {
			r.Post("/", h.V2AddToWatchlist)
			r.Delete("/{contractId}", h.V2RemoveFromWatchlist)
			r.Get("/", h.V2ListWatchlist)
			r.Get("/{contractId}/status", h.V2WatchlistStatus)
		})

		// Watchdog
		get("/watchdog/stats", h.V2WatchdogStats)
		get("/watchdog/alerts", h.V2ListWatchdogAlerts)
		get("/watchdog/contracts", h.V2ListMonitoredContracts)
		get("/watchdog/contracts/{id}", h.V2GetMonitoredContract)
		get("/watchdog/contracts/{id}/health", h.V2ListHealthChecks)
		get("/watchdog/contracts/{id}/alerts", h.V2ListWatchdogAlerts)
	})

	return r
}
