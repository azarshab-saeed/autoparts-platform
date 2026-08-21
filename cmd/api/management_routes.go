package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/example/autoparts-core/internal/management"
	"github.com/example/autoparts-core/internal/platform/api"
	"github.com/example/autoparts-core/internal/platform/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

func registerManagementRoutes(protected *http.ServeMux, pool *pgxpool.Pool) {
	svc := management.NewService(pool)
	protected.Handle("GET /v1/management/overview", auth.RequireRoles("owner", "admin", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		days := 30
		if raw := strings.TrimSpace(r.URL.Query().Get("days")); raw != "" {
			v, err := strconv.Atoi(raw)
			if err != nil || v < 7 || v > 180 {
				api.WriteError(w, api.BadRequest("invalid_days", "days must be between 7 and 180"))
				return
			}
			days = v
		}
		out, err := svc.Overview(r.Context(), c.TenantID, c.StoreID, days, time.Now())
		if err != nil {
			api.WriteError(w, api.Conflict("management_overview_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})))
}
