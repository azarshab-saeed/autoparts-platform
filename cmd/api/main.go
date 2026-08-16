package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/example/autoparts-core/internal/catalog"
	"github.com/example/autoparts-core/internal/customers"
	"github.com/example/autoparts-core/internal/finance"
	"github.com/example/autoparts-core/internal/inventory"
	"github.com/example/autoparts-core/internal/network"
	"github.com/example/autoparts-core/internal/platform/api"
	"github.com/example/autoparts-core/internal/platform/auth"
	"github.com/example/autoparts-core/internal/platform/db"
	"github.com/example/autoparts-core/internal/platform/pagination"
	"github.com/example/autoparts-core/internal/purchases"
	"github.com/example/autoparts-core/internal/reservations"
	returnsvc "github.com/example/autoparts-core/internal/returns"
	"github.com/example/autoparts-core/internal/sales"
	"github.com/example/autoparts-core/internal/stores"
	"github.com/example/autoparts-core/internal/suppliers"
	"github.com/google/uuid"
)

func main() {
	pool, err := db.New(nilContext(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	verifier, err := auth.NewKeycloakVerifier(
		os.Getenv("KEYCLOAK_ISSUER"),
		os.Getenv("KEYCLOAK_AUDIENCE"),
		os.Getenv("KEYCLOAK_JWKS_URL"),
	)
	if err != nil {
		log.Fatal(err)
	}

	catalogSvc := catalog.NewService(pool)
	customerSvc := customers.NewService(pool)
	supplierSvc := suppliers.NewService(pool)
	storeSvc := stores.NewService(pool)
	salesSvc := sales.NewService(pool)
	purchaseSvc := purchases.NewService(pool)
	inventorySvc := inventory.NewService(pool)
	networkSvc := network.NewService(pool)
	financeSvc := finance.NewService(pool)
	returnSvc := returnsvc.NewService(pool)
	reservationSvc := reservations.NewService(pool)

	public := http.NewServeMux()
	protected := http.NewServeMux()

	public.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := timeout(r, 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			api.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "down"})
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	protected.HandleFunc("GET /v1/me", func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		if c.Role == "mechanic" || c.Role == "consumer" {
			api.WriteJSON(w, http.StatusOK, map[string]any{
				"user_id": c.UserID, "role": c.Role, "roles": c.Roles,
				"display_name": c.Name, "email": c.Email,
			})
			return
		}
		out, err := storeSvc.Context(r.Context(), c.UserID, c.TenantID, c.StoreID, c.Role)
		if err != nil {
			api.WriteError(w, api.NotFound("store_context_not_found", "authenticated store context was not found"))
			return
		}
		out.Roles = c.Roles
		out.DisplayName = c.Name
		out.Email = c.Email
		api.WriteJSON(w, http.StatusOK, out)
	})

	protected.HandleFunc("GET /v1/products", func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		limit, offset, err := pageParams(r)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		out, e := catalogSvc.List(r.Context(), c.TenantID, r.URL.Query().Get("q"), limit, offset)
		if e != nil {
			api.WriteError(w, e)
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})
	protected.Handle("POST /v1/products", auth.RequireRoles("owner", "admin", "warehouse")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var in catalog.CreateProduct
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		if strings.TrimSpace(in.Title) == "" {
			api.WriteError(w, api.BadRequest("validation_error", "title is required"))
			return
		}
		out, err := catalogSvc.Create(r.Context(), c.TenantID, in)
		if err != nil {
			api.WriteError(w, api.Conflict("product_create_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

	protected.HandleFunc("GET /v1/customers", func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		limit, offset, err := pageParams(r)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		out, e := customerSvc.List(r.Context(), c.TenantID, c.StoreID, r.URL.Query().Get("q"), limit, offset)
		if e != nil {
			api.WriteError(w, e)
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})
	protected.Handle("POST /v1/customers", auth.RequireRoles("owner", "admin", "cashier", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var in customers.CreateCustomer
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		if strings.TrimSpace(in.Name) == "" {
			api.WriteError(w, api.BadRequest("validation_error", "name is required"))
			return
		}
		out, err := customerSvc.Create(r.Context(), c.TenantID, c.StoreID, in)
		if err != nil {
			api.WriteError(w, api.Conflict("customer_create_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

	protected.HandleFunc("GET /v1/suppliers", func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		limit, offset, err := pageParams(r)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		out, e := supplierSvc.List(r.Context(), c.TenantID, c.StoreID, r.URL.Query().Get("q"), limit, offset)
		if e != nil {
			api.WriteError(w, e)
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})
	protected.Handle("POST /v1/suppliers", auth.RequireRoles("owner", "admin", "warehouse", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var in suppliers.CreateSupplier
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		if strings.TrimSpace(in.Name) == "" {
			api.WriteError(w, api.BadRequest("validation_error", "name is required"))
			return
		}
		out, err := supplierSvc.Create(r.Context(), c.TenantID, c.StoreID, in)
		if err != nil {
			api.WriteError(w, api.Conflict("supplier_create_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

	protected.Handle("POST /v1/purchases", auth.RequireRoles("owner", "admin", "warehouse", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var cmd purchases.CreateCommand
		if err := decodeJSON(r, &cmd); err != nil {
			api.WriteError(w, err)
			return
		}
		cmd.TenantID, cmd.StoreID = c.TenantID, c.StoreID
		cmd.IdempotencyKey = r.Header.Get("Idempotency-Key")
		if cmd.IdempotencyKey == "" {
			api.WriteError(w, api.BadRequest("missing_idempotency_key", "Idempotency-Key header is required"))
			return
		}
		out, err := purchaseSvc.Create(r.Context(), cmd)
		if err != nil {
			api.WriteError(w, api.Conflict("purchase_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

	protected.HandleFunc("GET /v1/inventory", func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		limit, offset, err := pageParams(r)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		warehouseID, err := uuidFromQuery(r, "warehouse_id")
		if err != nil {
			api.WriteError(w, err)
			return
		}
		lowOnly := r.URL.Query().Get("low_stock") == "true"
		out, err := inventorySvc.List(r.Context(), c.TenantID, c.StoreID, warehouseID, lowOnly, limit, offset)
		if err != nil {
			api.WriteError(w, api.Conflict("inventory_query_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
	})

	protected.Handle("PUT /v1/inventory/reorder-point", auth.RequireRoles("owner", "admin", "warehouse")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var in struct {
			WarehouseID uuid.UUID `json:"warehouse_id"`
			ProductID   uuid.UUID `json:"product_id"`
			MinQty      float64   `json:"min_qty"`
			TargetQty   float64   `json:"target_qty"`
		}
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		if err := inventorySvc.SetReorderPoint(r.Context(), c.TenantID, c.StoreID, in.WarehouseID, in.ProductID, in.MinQty, in.TargetQty); err != nil {
			api.WriteError(w, api.Conflict("reorder_point_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})))

	protected.Handle("POST /v1/inventory/adjustments", auth.RequireRoles("owner", "admin", "warehouse")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var cmd inventory.AdjustmentCommand
		if err := decodeJSON(r, &cmd); err != nil {
			api.WriteError(w, err)
			return
		}
		cmd.TenantID, cmd.StoreID = c.TenantID, c.StoreID
		cmd.IdempotencyKey = r.Header.Get("Idempotency-Key")
		if cmd.IdempotencyKey == "" {
			api.WriteError(w, api.BadRequest("missing_idempotency_key", "Idempotency-Key header is required"))
			return
		}
		id, err := inventorySvc.Adjust(r.Context(), cmd)
		if err != nil {
			api.WriteError(w, api.Conflict("inventory_adjustment_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, map[string]any{"id": id, "status": "posted"})
	})))

	protected.HandleFunc("GET /v1/accounts/customers", func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		limit, offset, err := pageParams(r)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := financeSvc.ListCustomerBalances(r.Context(), c.TenantID, c.StoreID, r.URL.Query().Get("q"), limit, offset)
		if err != nil {
			api.WriteError(w, api.Conflict("customer_balances_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
	})
	protected.HandleFunc("GET /v1/accounts/suppliers", func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		limit, offset, err := pageParams(r)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := financeSvc.ListSupplierBalances(r.Context(), c.TenantID, c.StoreID, r.URL.Query().Get("q"), limit, offset)
		if err != nil {
			api.WriteError(w, api.Conflict("supplier_balances_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
	})
	protected.Handle("POST /v1/settlements/customer-receipts", auth.RequireRoles("owner", "admin", "cashier", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var cmd finance.SettlementCommand
		if err := decodeJSON(r, &cmd); err != nil {
			api.WriteError(w, err)
			return
		}
		cmd.TenantID, cmd.StoreID, cmd.PartyType, cmd.IdempotencyKey = c.TenantID, c.StoreID, "customer", r.Header.Get("Idempotency-Key")
		if cmd.IdempotencyKey == "" {
			api.WriteError(w, api.BadRequest("missing_idempotency_key", "Idempotency-Key header is required"))
			return
		}
		out, err := financeSvc.CreateSettlement(r.Context(), cmd)
		if err != nil {
			api.WriteError(w, api.Conflict("customer_receipt_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))
	protected.Handle("POST /v1/settlements/supplier-payments", auth.RequireRoles("owner", "admin", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var cmd finance.SettlementCommand
		if err := decodeJSON(r, &cmd); err != nil {
			api.WriteError(w, err)
			return
		}
		cmd.TenantID, cmd.StoreID, cmd.PartyType, cmd.IdempotencyKey = c.TenantID, c.StoreID, "supplier", r.Header.Get("Idempotency-Key")
		if cmd.IdempotencyKey == "" {
			api.WriteError(w, api.BadRequest("missing_idempotency_key", "Idempotency-Key header is required"))
			return
		}
		out, err := financeSvc.CreateSettlement(r.Context(), cmd)
		if err != nil {
			api.WriteError(w, api.Conflict("supplier_payment_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))
	protected.HandleFunc("GET /v1/sales/{id}", func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_sale_id", "sale id must be a UUID"))
			return
		}
		out, err := salesSvc.Detail(r.Context(), c.TenantID, c.StoreID, id)
		if err != nil {
			api.WriteError(w, api.NotFound("sale_not_found", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})
	protected.HandleFunc("GET /v1/purchases/{id}", func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_purchase_id", "purchase id must be a UUID"))
			return
		}
		out, err := purchaseSvc.Detail(r.Context(), c.TenantID, c.StoreID, id)
		if err != nil {
			api.WriteError(w, api.NotFound("purchase_not_found", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})
	protected.Handle("POST /v1/returns/sales", auth.RequireRoles("owner", "admin", "cashier")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var cmd returnsvc.SaleReturnCommand
		if err := decodeJSON(r, &cmd); err != nil {
			api.WriteError(w, err)
			return
		}
		cmd.TenantID, cmd.StoreID, cmd.IdempotencyKey = c.TenantID, c.StoreID, r.Header.Get("Idempotency-Key")
		if cmd.IdempotencyKey == "" {
			api.WriteError(w, api.BadRequest("missing_idempotency_key", "Idempotency-Key header is required"))
			return
		}
		out, err := returnSvc.CreateSaleReturn(r.Context(), cmd)
		if err != nil {
			api.WriteError(w, api.Conflict("sale_return_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))
	protected.Handle("POST /v1/returns/purchases", auth.RequireRoles("owner", "admin", "warehouse", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var cmd returnsvc.PurchaseReturnCommand
		if err := decodeJSON(r, &cmd); err != nil {
			api.WriteError(w, err)
			return
		}
		cmd.TenantID, cmd.StoreID, cmd.IdempotencyKey = c.TenantID, c.StoreID, r.Header.Get("Idempotency-Key")
		if cmd.IdempotencyKey == "" {
			api.WriteError(w, api.BadRequest("missing_idempotency_key", "Idempotency-Key header is required"))
			return
		}
		out, err := returnSvc.CreatePurchaseReturn(r.Context(), cmd)
		if err != nil {
			api.WriteError(w, api.Conflict("purchase_return_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

	protected.Handle("GET /v1/network/store-profile", auth.RequireRoles("owner", "admin", "warehouse")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		out, err := networkSvc.GetStoreProfile(r.Context(), c.TenantID, c.StoreID)
		if err != nil {
			api.WriteError(w, api.NotFound("network_store_profile_not_found", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})))
	protected.Handle("PUT /v1/network/store-profile", auth.RequireRoles("owner", "admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var in network.UpdateStoreProfile
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		if err := networkSvc.UpdateStoreProfile(r.Context(), c.TenantID, c.StoreID, in); err != nil {
			api.WriteError(w, api.Conflict("network_store_profile_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})))

	protected.Handle("GET /v1/network/offers", auth.RequireRoles("owner", "admin", "warehouse")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		warehouseID, err := uuidFromQuery(r, "warehouse_id")
		if err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := networkSvc.ListStoreOffers(r.Context(), c.TenantID, c.StoreID, warehouseID)
		if err != nil {
			api.WriteError(w, api.Conflict("network_offers_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
	})))
	protected.Handle("PUT /v1/network/offers/{product_id}", auth.RequireRoles("owner", "admin", "warehouse")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		productID, err := uuid.Parse(r.PathValue("product_id"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_product_id", "product id must be a UUID"))
			return
		}
		var in network.UpdateOffer
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		if err := networkSvc.UpsertOffer(r.Context(), c.TenantID, c.StoreID, productID, in); err != nil {
			api.WriteError(w, api.Conflict("network_offer_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})))

	protected.Handle("POST /v1/network/reservations", auth.RequireRoles("mechanic", "consumer")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var in struct {
			OfferID uuid.UUID `json:"offer_id"`
			Qty     float64   `json:"qty"`
		}
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			api.WriteError(w, api.BadRequest("missing_idempotency_key", "Idempotency-Key header is required"))
			return
		}
		out, err := reservationSvc.Create(r.Context(), reservations.CreateCommand{OfferID: in.OfferID, Qty: in.Qty, BuyerUserID: c.UserID, BuyerName: c.Name, BuyerEmail: c.Email, IdempotencyKey: key})
		if err != nil {
			api.WriteError(w, api.Conflict("reservation_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

	protected.Handle("GET /v1/me/reservations", auth.RequireRoles("mechanic", "consumer")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		out, err := reservationSvc.ListBuyer(r.Context(), c.UserID)
		if err != nil {
			api.WriteError(w, api.Conflict("reservations_query_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
	})))

	protected.Handle("POST /v1/me/reservations/{id}/cancel", auth.RequireRoles("mechanic", "consumer")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_reservation_id", "reservation id must be a UUID"))
			return
		}
		out, err := reservationSvc.CancelBuyer(r.Context(), c.UserID, id)
		if err != nil {
			api.WriteError(w, api.Conflict("reservation_cancel_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})))

	protected.Handle("GET /v1/network/reservations", auth.RequireRoles("owner", "admin", "cashier", "warehouse")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		out, err := reservationSvc.ListStore(r.Context(), c.TenantID, c.StoreID, strings.TrimSpace(r.URL.Query().Get("status")))
		if err != nil {
			api.WriteError(w, api.Conflict("store_reservations_query_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
	})))

	protected.Handle("PATCH /v1/network/reservations/{id}", auth.RequireRoles("owner", "admin", "cashier", "warehouse")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_reservation_id", "reservation id must be a UUID"))
			return
		}
		var in struct {
			Status string `json:"status"`
		}
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := reservationSvc.StoreTransition(r.Context(), c.TenantID, c.StoreID, c.UserID, id, strings.TrimSpace(in.Status))
		if err != nil {
			api.WriteError(w, api.Conflict("reservation_transition_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})))

	protected.Handle("POST /v1/network/reservations/{id}/fulfill", auth.RequireRoles("owner", "admin", "cashier")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_reservation_id", "reservation id must be a UUID"))
			return
		}
		var in struct {
			CustomerID    *uuid.UUID          `json:"customer_id,omitempty"`
			PaymentMethod string              `json:"payment_method,omitempty"`
			Payments      []sales.PaymentPart `json:"payments,omitempty"`
		}
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			api.WriteError(w, api.BadRequest("missing_idempotency_key", "Idempotency-Key header is required"))
			return
		}
		out, err := salesSvc.FulfillReservation(r.Context(), sales.FulfillReservationCommand{
			TenantID: c.TenantID, StoreID: c.StoreID, ActorUserID: c.UserID, ReservationID: id,
			CustomerID: in.CustomerID, PaymentMethod: strings.TrimSpace(in.PaymentMethod), Payments: in.Payments, IdempotencyKey: key,
		})
		if err != nil {
			api.WriteError(w, api.Conflict("reservation_fulfillment_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

	protected.Handle("POST /v1/sales", auth.RequireRoles("owner", "admin", "cashier")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var cmd sales.CreateSaleCommand
		if err := decodeJSON(r, &cmd); err != nil {
			api.WriteError(w, err)
			return
		}
		cmd.TenantID = c.TenantID
		cmd.StoreID = c.StoreID
		cmd.IdempotencyKey = r.Header.Get("Idempotency-Key")
		if cmd.IdempotencyKey == "" {
			api.WriteError(w, api.BadRequest("missing_idempotency_key", "Idempotency-Key header is required"))
			return
		}
		out, err := salesSvc.Create(r.Context(), cmd)
		if err != nil {
			api.WriteError(w, api.Conflict("sale_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

	protectedHandler := auth.Middleware(verifier, protected)
	root := http.NewServeMux()
	root.HandleFunc("GET /v1/network/search", func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		lat, err := optionalFloatQuery(r, "lat", -90, 90)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		lng, err := optionalFloatQuery(r, "lng", -180, 180)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		if (lat == nil) != (lng == nil) {
			api.WriteError(w, api.BadRequest("invalid_location", "lat and lng must be provided together"))
			return
		}
		limit, _, err := pageParams(r)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		_, _ = reservationSvc.ExpireDue(r.Context(), 100)
		out, err := networkSvc.Search(r.Context(), q, lat, lng, r.URL.Query().Get("sort"), limit)
		if err != nil {
			api.WriteError(w, api.BadRequest("network_search_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": out, "count": len(out)})
	})
	root.Handle("/v1/", protectedHandler)
	root.Handle("/healthz", public)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{Addr: ":" + port, Handler: requestLog(cors(root)), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 90 * time.Second}
	log.Printf("api listening on :%s", port)
	log.Fatal(srv.ListenAndServe())
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return api.BadRequest("invalid_json", "request body is not valid JSON")
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return api.BadRequest("invalid_json", "request body must contain one JSON object")
	}
	return nil
}

func pageParams(r *http.Request) (int, int, error) {
	limit, err := pagination.Limit(r.URL.Query().Get("limit"))
	if err != nil {
		return 0, 0, api.BadRequest("invalid_limit", err.Error())
	}
	offset, err := pagination.DecodeOffset(r.URL.Query().Get("cursor"))
	if err != nil {
		return 0, 0, api.BadRequest("invalid_cursor", err.Error())
	}
	return limit, offset, nil
}

func uuidFromQuery(r *http.Request, name string) (uuid.UUID, error) {
	v := strings.TrimSpace(r.URL.Query().Get(name))
	if v == "" {
		return uuid.Nil, api.BadRequest("missing_"+name, name+" is required")
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil, api.BadRequest("invalid_"+name, name+" must be a UUID")
	}
	return id, nil
}

func optionalFloatQuery(r *http.Request, name string, min, max float64) (*float64, error) {
	v := strings.TrimSpace(r.URL.Query().Get(name))
	if v == "" {
		return nil, nil
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil || n < min || n > max {
		return nil, api.BadRequest("invalid_"+name, name+" is outside its valid range")
	}
	return &n, nil
}

func timeout(r *http.Request, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), d)
}
func nilContext() context.Context { return context.Background() }

func cors(next http.Handler) http.Handler {
	allowed := strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS"))
	if allowed == "" {
		allowed = "http://localhost:3000"
	}
	set := map[string]bool{}
	for _, origin := range strings.Split(allowed, ",") {
		set[strings.TrimSpace(origin)] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && set[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Bootstrap-Secret")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(started).Round(time.Millisecond))
	})
}
