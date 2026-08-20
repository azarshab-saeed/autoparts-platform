package main

import (
	"net/http"
	"strconv"

	"github.com/example/autoparts-core/internal/platform/api"
	"github.com/example/autoparts-core/internal/platform/auth"
	"github.com/example/autoparts-core/internal/vehiclenotebook"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func registerVehicleNotebookRoutes(public, protected *http.ServeMux, pool *pgxpool.Pool) {
	svc := vehiclenotebook.NewService(pool)

	public.HandleFunc("GET /v1/public/vehicle-notebook/{token}", func(w http.ResponseWriter, r *http.Request) {
		token, err := uuid.Parse(r.PathValue("token"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_vehicle_token", "vehicle QR token is invalid"))
			return
		}
		out, err := svc.PublicByToken(r.Context(), token)
		if err != nil {
			api.WriteError(w, api.NotFound("vehicle_notebook_not_found", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})

	public.HandleFunc("POST /v1/public/vehicle-notebook/{token}/mileage", func(w http.ResponseWriter, r *http.Request) {
		token, err := uuid.Parse(r.PathValue("token"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_vehicle_token", "vehicle QR token is invalid"))
			return
		}
		var in vehiclenotebook.OwnerMileageInput
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := svc.AddOwnerMileage(r.Context(), token, in)
		if err != nil {
			api.WriteError(w, api.Conflict("owner_mileage_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})

	protected.Handle("GET /v1/vehicle-notebook", auth.RequireRoles("owner", "admin", "cashier", "warehouse", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		limit := 40
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil {
				limit = parsed
			}
		}
		items, err := svc.List(r.Context(), c.TenantID, c.StoreID, r.URL.Query().Get("q"), limit)
		if err != nil {
			api.WriteError(w, api.Conflict("vehicle_notebook_list_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})))

	protected.Handle("POST /v1/vehicle-notebook", auth.RequireRoles("owner", "admin", "cashier")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var in vehiclenotebook.CreateVehicle
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := svc.Create(r.Context(), c.TenantID, c.StoreID, c.UserID, in)
		if err != nil {
			api.WriteError(w, api.Conflict("vehicle_notebook_create_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

	protected.Handle("GET /v1/vehicle-notebook/by-token/{token}", auth.RequireRoles("owner", "admin", "cashier", "warehouse", "accountant", "mechanic", "consumer")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		token, err := uuid.Parse(r.PathValue("token"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_vehicle_token", "vehicle QR token is invalid"))
			return
		}
		out, err := svc.ByToken(r.Context(), token, c.TenantID, c.StoreID)
		if err != nil {
			api.WriteError(w, api.NotFound("vehicle_notebook_not_found", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})))

	protected.Handle("POST /v1/vehicle-notebook/by-token/{token}/entries", auth.RequireRoles("owner", "admin", "cashier", "warehouse", "accountant", "mechanic")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		token, err := uuid.Parse(r.PathValue("token"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_vehicle_token", "vehicle QR token is invalid"))
			return
		}
		var in vehiclenotebook.AddEntry
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := svc.AddEntryByToken(r.Context(), token, c.UserID, c.Role, c.Name, c.TenantID, c.StoreID, in)
		if err != nil {
			api.WriteError(w, api.Conflict("vehicle_notebook_entry_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

	protected.Handle("POST /v1/vehicle-notebook/{id}/owner-code", auth.RequireRoles("owner", "admin", "cashier")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_vehicle_id", "vehicle id must be a UUID"))
			return
		}
		code, err := svc.RotateOwnerCode(r.Context(), c.TenantID, c.StoreID, id)
		if err != nil {
			api.WriteError(w, api.Conflict("owner_code_rotate_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"owner_code": code})
	})))
}
