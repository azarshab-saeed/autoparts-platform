package main

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/example/autoparts-core/internal/platform/api"
	"github.com/example/autoparts-core/internal/platform/auth"
	"github.com/example/autoparts-core/internal/tradeaccount"
	"github.com/example/autoparts-core/internal/vehiclenotebook"
	"github.com/example/autoparts-core/internal/workshop"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func registerWorkshopNetworkRoutes(protected *http.ServeMux, pool *pgxpool.Pool) {
	vehicleSvc := vehiclenotebook.NewService(pool)
	workshopSvc := workshop.NewService(pool)
	tradeSvc := tradeaccount.NewService(pool)

	protected.Handle("GET /v1/mechanic/vehicles", auth.RequireRoles("mechanic")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		items, err := vehicleSvc.ListExternal(r.Context(), c.UserID, r.URL.Query().Get("q"), 60)
		if err != nil {
			api.WriteError(w, api.Conflict("mechanic_vehicle_list_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})))

	protected.Handle("POST /v1/mechanic/vehicles", auth.RequireRoles("mechanic")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var in vehiclenotebook.CreateVehicle
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := vehicleSvc.CreateExternal(r.Context(), c.UserID, c.Role, in)
		if err != nil {
			api.WriteError(w, api.Conflict("mechanic_vehicle_create_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

	protected.Handle("POST /v1/mechanic/vehicles/{id}/owner-code", auth.RequireRoles("mechanic")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_vehicle_id", "vehicle id must be a UUID"))
			return
		}
		code, err := vehicleSvc.RotateExternalOwnerCode(r.Context(), c.UserID, id)
		if err != nil {
			api.WriteError(w, api.Conflict("owner_code_rotate_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]string{"owner_code": code})
	})))

	protected.Handle("GET /v1/mechanic/workshop/summary", auth.RequireRoles("mechanic")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		out, err := workshopSvc.Summary(r.Context(), c.UserID)
		if err != nil {
			api.WriteError(w, api.Conflict("workshop_summary_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})))

	protected.Handle("GET /v1/mechanic/workshop/jobs", auth.RequireRoles("mechanic")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		items, err := workshopSvc.ListJobs(r.Context(), c.UserID, r.URL.Query().Get("status"), 80)
		if err != nil {
			api.WriteError(w, api.BadRequest("workshop_jobs_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})))

	protected.Handle("POST /v1/mechanic/workshop/jobs", auth.RequireRoles("mechanic")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var in workshop.CreateJob
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := workshopSvc.CreateJob(r.Context(), c.UserID, in)
		if err != nil {
			api.WriteError(w, api.Conflict("workshop_job_create_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

	protected.Handle("GET /v1/mechanic/workshop/jobs/{id}", auth.RequireRoles("mechanic")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := routeUUID(r, "id")
		if err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := workshopSvc.GetJob(r.Context(), c.UserID, id)
		if err != nil {
			api.WriteError(w, api.NotFound("workshop_job_not_found", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})))

	protected.Handle("POST /v1/mechanic/workshop/jobs/{id}/items", auth.RequireRoles("mechanic")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := routeUUID(r, "id")
		if err != nil {
			api.WriteError(w, err)
			return
		}
		var in workshop.AddItem
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := workshopSvc.AddItem(r.Context(), c.UserID, id, in)
		if err != nil {
			api.WriteError(w, api.Conflict("workshop_item_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

	protected.Handle("POST /v1/mechanic/workshop/jobs/{id}/payments", auth.RequireRoles("mechanic")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := routeUUID(r, "id")
		if err != nil {
			api.WriteError(w, err)
			return
		}
		var in workshop.AddPayment
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := workshopSvc.AddPayment(r.Context(), c.UserID, id, in)
		if err != nil {
			api.WriteError(w, api.Conflict("workshop_payment_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

	protected.Handle("POST /v1/mechanic/workshop/jobs/{id}/complete", auth.RequireRoles("mechanic")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := routeUUID(r, "id")
		if err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := workshopSvc.Complete(r.Context(), c.UserID, id, c.Name)
		if err != nil {
			api.WriteError(w, api.Conflict("workshop_complete_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})))

	protected.Handle("GET /v1/mechanic/trade-accounts", auth.RequireRoles("mechanic")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		items, err := tradeSvc.ListMechanicAccounts(r.Context(), c.UserID)
		if err != nil {
			api.WriteError(w, api.Conflict("mechanic_accounts_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})))

	protected.Handle("GET /v1/mechanic/trade-accounts/{id}/ledger", auth.RequireRoles("mechanic")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := routeUUID(r, "id")
		if err != nil {
			api.WriteError(w, err)
			return
		}
		items, err := tradeSvc.LedgerForMechanic(r.Context(), c.UserID, id)
		if err != nil {
			api.WriteError(w, api.NotFound("mechanic_ledger_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})))

	protected.Handle("GET /v1/mechanic/trade-requests", auth.RequireRoles("mechanic")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		items, err := tradeSvc.ListMechanicRequests(r.Context(), c.UserID, r.URL.Query().Get("status"))
		if err != nil {
			api.WriteError(w, api.BadRequest("mechanic_trade_requests_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})))

	protected.Handle("POST /v1/mechanic/reservations/{id}/trade-charge", auth.RequireRoles("mechanic")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := routeUUID(r, "id")
		if err != nil {
			api.WriteError(w, err)
			return
		}
		var in struct {
			Note string `json:"note,omitempty"`
		}
		if r.ContentLength > 0 {
			if err := decodeJSON(r, &in); err != nil {
				api.WriteError(w, err)
				return
			}
		}
		out, err := tradeSvc.CreateReservationCharge(r.Context(), c.UserID, c.Name, c.Email, id, in.Note)
		if err != nil {
			api.WriteError(w, api.Conflict("reservation_trade_charge_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

	protected.Handle("POST /v1/mechanic/trade-accounts/{id}/settlements", auth.RequireRoles("mechanic")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := routeUUID(r, "id")
		if err != nil {
			api.WriteError(w, err)
			return
		}
		var in tradeaccount.CreateRequest
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := tradeSvc.CreateMechanicPayment(r.Context(), c.UserID, id, in)
		if err != nil {
			api.WriteError(w, api.Conflict("mechanic_settlement_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

	protected.Handle("POST /v1/mechanic/trade-requests/{id}/resolve", auth.RequireRoles("mechanic")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := routeUUID(r, "id")
		if err != nil {
			api.WriteError(w, err)
			return
		}
		confirm, err := resolveInput(r)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := tradeSvc.ResolveByMechanic(r.Context(), c.UserID, id, confirm)
		if err != nil {
			api.WriteError(w, api.Conflict("mechanic_trade_resolve_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})))

	storeRoles := auth.RequireRoles("owner", "admin", "cashier", "accountant")
	protected.Handle("GET /v1/store/mechanic-accounts", storeRoles(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		items, err := tradeSvc.ListStoreAccounts(r.Context(), c.TenantID, c.StoreID)
		if err != nil {
			api.WriteError(w, api.Conflict("store_mechanic_accounts_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})))

	protected.Handle("GET /v1/store/mechanic-accounts/{id}/ledger", storeRoles(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := routeUUID(r, "id")
		if err != nil {
			api.WriteError(w, err)
			return
		}
		items, err := tradeSvc.LedgerForStore(r.Context(), c.TenantID, c.StoreID, id)
		if err != nil {
			api.WriteError(w, api.NotFound("store_mechanic_ledger_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})))

	protected.Handle("GET /v1/store/mechanic-trade-requests", storeRoles(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		items, err := tradeSvc.ListStoreRequests(r.Context(), c.TenantID, c.StoreID, r.URL.Query().Get("status"))
		if err != nil {
			api.WriteError(w, api.BadRequest("store_mechanic_requests_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})))

	protected.Handle("POST /v1/store/mechanic-accounts/{id}/requests", storeRoles(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := routeUUID(r, "id")
		if err != nil {
			api.WriteError(w, err)
			return
		}
		var in tradeaccount.CreateRequest
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := tradeSvc.CreateStoreRequest(r.Context(), c.TenantID, c.StoreID, c.UserID, id, in)
		if err != nil {
			api.WriteError(w, api.Conflict("store_mechanic_request_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

	protected.Handle("POST /v1/store/mechanic-trade-requests/{id}/resolve", storeRoles(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := routeUUID(r, "id")
		if err != nil {
			api.WriteError(w, err)
			return
		}
		confirm, err := resolveInput(r)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := tradeSvc.ResolveByStore(r.Context(), c.TenantID, c.StoreID, c.UserID, id, confirm)
		if err != nil {
			api.WriteError(w, api.Conflict("store_mechanic_resolve_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})))
}

func routeUUID(r *http.Request, key string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(r.PathValue(key)))
	if err != nil {
		return uuid.Nil, api.BadRequest("invalid_"+key, key+" must be a UUID")
	}
	return id, nil
}

func resolveInput(r *http.Request) (bool, error) {
	var in struct {
		Action string `json:"action"`
	}
	if err := decodeJSON(r, &in); err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(in.Action)) {
	case "confirm":
		return true, nil
	case "reject":
		return false, nil
	default:
		return false, api.BadRequest("invalid_action", "action must be confirm or reject")
	}
}

func boundedInt(raw string, fallback, min, max int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v < min || v > max {
		return fallback
	}
	return v
}
