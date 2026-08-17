package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/example/autoparts-core/internal/catalog"
	"github.com/example/autoparts-core/internal/customers"
	"github.com/example/autoparts-core/internal/edge"
	"github.com/example/autoparts-core/internal/finance"
	"github.com/example/autoparts-core/internal/fitment"
	"github.com/example/autoparts-core/internal/inventory"
	"github.com/example/autoparts-core/internal/network"
	"github.com/example/autoparts-core/internal/operations"
	"github.com/example/autoparts-core/internal/platform/api"
	"github.com/example/autoparts-core/internal/platform/audit"
	"github.com/example/autoparts-core/internal/platform/auth"
	"github.com/example/autoparts-core/internal/platform/db"
	"github.com/example/autoparts-core/internal/platform/httpx"
	"github.com/example/autoparts-core/internal/platform/pagination"
	"github.com/example/autoparts-core/internal/procurement"
	"github.com/example/autoparts-core/internal/purchases"
	"github.com/example/autoparts-core/internal/reservations"
	returnsvc "github.com/example/autoparts-core/internal/returns"
	"github.com/example/autoparts-core/internal/sales"
	"github.com/example/autoparts-core/internal/stores"
	"github.com/example/autoparts-core/internal/suppliers"
	"github.com/google/uuid"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

const latestMigration = "014_store_edge_offline.sql"

func main() {
	log.SetFlags(0)
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
	procurementSvc := procurement.NewService(pool)
	inventorySvc := inventory.NewService(pool)
	networkSvc := network.NewService(pool)
	financeSvc := finance.NewService(pool)
	fitmentSvc := fitment.NewService(pool)
	returnSvc := returnsvc.NewService(pool)
	reservationSvc := reservations.NewService(pool)
	operationsSvc := operations.NewService(pool)
	auditSvc := audit.NewService(pool)
	edgeSvc := edge.NewService(pool)

	public := http.NewServeMux()
	protected := http.NewServeMux()

	public.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		api.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "autoparts-api", "version": version})
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
	protected.HandleFunc("GET /v1/products/{product_id}/search-metadata", func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		productID, err := uuid.Parse(r.PathValue("product_id"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_product_id", "product id must be a UUID"))
			return
		}
		out, err := fitmentSvc.GetProductMetadata(r.Context(), c.TenantID, productID)
		if err != nil {
			api.WriteError(w, api.NotFound("product_search_metadata_not_found", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})
	protected.Handle("PUT /v1/products/{product_id}/search-metadata", auth.RequireRoles("owner", "admin", "warehouse")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		productID, err := uuid.Parse(r.PathValue("product_id"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_product_id", "product id must be a UUID"))
			return
		}
		var in fitment.UpdateMetadata
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := fitmentSvc.UpdateProductMetadata(r.Context(), c.TenantID, productID, in)
		if err != nil {
			api.WriteError(w, api.Conflict("product_search_metadata_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})))

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

	protected.Handle("POST /v1/products/import", auth.RequireRoles("owner", "admin", "warehouse")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			api.WriteError(w, api.BadRequest("missing_idempotency_key", "Idempotency-Key header is required"))
			return
		}
		var in struct {
			WarehouseID uuid.UUID           `json:"warehouse_id"`
			Rows        []catalog.ImportRow `json:"rows"`
		}
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := catalogSvc.Import(r.Context(), catalog.ImportCommand{
			TenantID: c.TenantID, StoreID: c.StoreID, WarehouseID: in.WarehouseID, ActorUserID: c.UserID,
			IdempotencyKey: key, Rows: in.Rows,
		})
		if err != nil {
			var validationErr *catalog.ImportValidationError
			if errors.As(err, &validationErr) {
				api.WriteJSON(w, http.StatusBadRequest, map[string]any{
					"error": map[string]any{"code": "import_validation_failed", "message": validationErr.Error(), "rows": validationErr.Rows},
				})
				return
			}
			api.WriteError(w, api.Conflict("catalog_import_failed", err.Error()))
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

	protected.Handle("GET /v1/accounts/customers/{id}/statement", auth.RequireRoles("owner", "admin", "cashier", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_customer_id", "customer id must be a UUID"))
			return
		}
		out, err := financeSvc.PartyStatement(r.Context(), c.TenantID, c.StoreID, "customer", id)
		if err != nil {
			api.WriteError(w, api.NotFound("customer_statement_not_found", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})))
	protected.Handle("GET /v1/accounts/suppliers/{id}/statement", auth.RequireRoles("owner", "admin", "cashier", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_supplier_id", "supplier id must be a UUID"))
			return
		}
		out, err := financeSvc.PartyStatement(r.Context(), c.TenantID, c.StoreID, "supplier", id)
		if err != nil {
			api.WriteError(w, api.NotFound("supplier_statement_not_found", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})))

	protected.Handle("GET /v1/dashboard", auth.RequireRoles("owner", "admin", "cashier", "warehouse", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		out, err := operationsSvc.Dashboard(r.Context(), c.TenantID, c.StoreID, time.Now())
		if err != nil {
			api.WriteError(w, api.Conflict("dashboard_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})))

	protected.Handle("GET /v1/sales", auth.RequireRoles("owner", "admin", "cashier", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		from, to, err := businessDateRange(r)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		limit, offset, err := pageParams(r)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		customerID, err := optionalUUIDQuery(r, "customer_id")
		if err != nil {
			api.WriteError(w, err)
			return
		}
		items, total, err := operationsSvc.ListSales(r.Context(), c.TenantID, c.StoreID, from, to, customerID, r.URL.Query().Get("q"), r.URL.Query().Get("payment_state"), limit, offset)
		if err != nil {
			api.WriteError(w, api.BadRequest("sales_history_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, pagedEnvelope(items, total, limit, offset))
	})))

	protected.Handle("GET /v1/purchases", auth.RequireRoles("owner", "admin", "warehouse", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		from, to, err := businessDateRange(r)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		limit, offset, err := pageParams(r)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		supplierID, err := optionalUUIDQuery(r, "supplier_id")
		if err != nil {
			api.WriteError(w, err)
			return
		}
		items, total, err := operationsSvc.ListPurchases(r.Context(), c.TenantID, c.StoreID, from, to, supplierID, r.URL.Query().Get("q"), r.URL.Query().Get("payment_state"), limit, offset)
		if err != nil {
			api.WriteError(w, api.BadRequest("purchases_history_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, pagedEnvelope(items, total, limit, offset))
	})))

	protected.Handle("GET /v1/expenses/categories", auth.RequireRoles("owner", "admin", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		out, err := financeSvc.ListExpenseCategories(r.Context(), c.TenantID)
		if err != nil {
			api.WriteError(w, api.Conflict("expense_categories_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
	})))
	protected.Handle("GET /v1/expenses", auth.RequireRoles("owner", "admin", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		from, to, err := businessDateRange(r)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		limit, offset, err := pageParams(r)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		var categoryID *uuid.UUID
		if raw := strings.TrimSpace(r.URL.Query().Get("category_id")); raw != "" {
			id, parseErr := uuid.Parse(raw)
			if parseErr != nil {
				api.WriteError(w, api.BadRequest("invalid_category_id", "category_id must be a UUID"))
				return
			}
			categoryID = &id
		}
		out, err := financeSvc.ListExpenses(r.Context(), c.TenantID, c.StoreID, from, to, categoryID, limit, offset)
		if err != nil {
			api.WriteError(w, api.Conflict("expenses_query_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
	})))
	protected.Handle("POST /v1/expenses", auth.RequireRoles("owner", "admin", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var cmd finance.ExpenseCommand
		if err := decodeJSON(r, &cmd); err != nil {
			api.WriteError(w, err)
			return
		}
		cmd.TenantID, cmd.StoreID = c.TenantID, c.StoreID
		cmd.IdempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if cmd.IdempotencyKey == "" {
			api.WriteError(w, api.BadRequest("missing_idempotency_key", "Idempotency-Key header is required"))
			return
		}
		out, err := financeSvc.CreateExpense(r.Context(), cmd)
		if err != nil {
			api.WriteError(w, api.Conflict("expense_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))
	protected.Handle("GET /v1/reports/profit-loss", auth.RequireRoles("owner", "admin", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		from, to, err := businessDateRange(r)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := financeSvc.ProfitLoss(r.Context(), c.TenantID, c.StoreID, from, to)
		if err != nil {
			api.WriteError(w, api.Conflict("profit_loss_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})))

	protected.Handle("GET /v1/reports/inventory", auth.RequireRoles("owner", "admin", "warehouse", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		warehouseID, err := uuidFromQuery(r, "warehouse_id")
		if err != nil {
			api.WriteError(w, err)
			return
		}
		limit, offset, err := pageParams(r)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := operationsSvc.InventoryInsights(r.Context(), c.TenantID, c.StoreID, warehouseID, r.URL.Query().Get("q"), r.URL.Query().Get("sort"), limit, offset)
		if err != nil {
			api.WriteError(w, api.BadRequest("inventory_report_failed", err.Error()))
			return
		}
		response := map[string]any{"summary": out.Summary, "items": out.Items, "total": out.Total, "next_cursor": ""}
		if offset+len(out.Items) < out.Total {
			response["next_cursor"] = pagination.EncodeOffset(offset + len(out.Items))
		}
		api.WriteJSON(w, http.StatusOK, response)
	})))

	protected.Handle("GET /v1/reports/cash", auth.RequireRoles("owner", "admin", "cashier", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		date, err := businessDateParam(r, "date", time.Now())
		if err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := operationsSvc.CashReport(r.Context(), c.TenantID, c.StoreID, date)
		if err != nil {
			api.WriteError(w, api.Conflict("cash_report_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})))

	protected.Handle("POST /v1/daily-closings", auth.RequireRoles("owner", "admin", "cashier", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var cmd operations.CloseDayCommand
		if err := decodeJSON(r, &cmd); err != nil {
			api.WriteError(w, err)
			return
		}
		cmd.TenantID, cmd.StoreID, cmd.ActorUserID = c.TenantID, c.StoreID, c.UserID
		cmd.IdempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if cmd.IdempotencyKey == "" {
			api.WriteError(w, api.BadRequest("missing_idempotency_key", "Idempotency-Key header is required"))
			return
		}
		out, err := operationsSvc.CloseDay(r.Context(), cmd)
		if err != nil {
			api.WriteError(w, api.Conflict("daily_closing_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

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
	protected.Handle("GET /v1/sales/{id}", auth.RequireRoles("owner", "admin", "cashier", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	})))
	protected.Handle("GET /v1/purchases/{id}", auth.RequireRoles("owner", "admin", "warehouse", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	})))
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

	protected.Handle("GET /v1/network/procurement/search", auth.RequireRoles("owner", "admin", "warehouse")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
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
		_, _ = procurementSvc.ExpireDue(r.Context(), 100)
		out, err := networkSvc.SearchProcurement(r.Context(), c.StoreID, q, lat, lng, r.URL.Query().Get("sort"), limit)
		if err != nil {
			api.WriteError(w, api.BadRequest("procurement_search_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": out, "count": len(out)})
	})))

	protected.Handle("POST /v1/network/procurements", auth.RequireRoles("owner", "admin", "warehouse")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var in struct {
			OfferID        uuid.UUID `json:"offer_id"`
			BuyerProductID uuid.UUID `json:"buyer_product_id"`
			WarehouseID    uuid.UUID `json:"warehouse_id"`
			Qty            float64   `json:"qty"`
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
		out, err := procurementSvc.Create(r.Context(), procurement.CreateCommand{
			BuyerTenantID: c.TenantID, BuyerStoreID: c.StoreID, BuyerWarehouseID: in.WarehouseID,
			BuyerProductID: in.BuyerProductID, ActorUserID: c.UserID, OfferID: in.OfferID, Qty: in.Qty, IdempotencyKey: key,
		})
		if err != nil {
			api.WriteError(w, api.Conflict("procurement_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))

	protected.Handle("GET /v1/network/procurements/buying", auth.RequireRoles("owner", "admin", "warehouse", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		out, err := procurementSvc.ListBuyer(r.Context(), c.TenantID, c.StoreID, strings.TrimSpace(r.URL.Query().Get("status")))
		if err != nil {
			api.WriteError(w, api.Conflict("procurement_buying_query_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
	})))

	protected.Handle("GET /v1/network/procurements/selling", auth.RequireRoles("owner", "admin", "warehouse", "accountant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		out, err := procurementSvc.ListSeller(r.Context(), c.TenantID, c.StoreID, strings.TrimSpace(r.URL.Query().Get("status")))
		if err != nil {
			api.WriteError(w, api.Conflict("procurement_selling_query_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
	})))

	protected.Handle("PATCH /v1/network/procurements/{id}", auth.RequireRoles("owner", "admin", "warehouse")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_procurement_id", "procurement id must be a UUID"))
			return
		}
		var in struct {
			Status string `json:"status"`
		}
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := procurementSvc.SellerTransition(r.Context(), c.TenantID, c.StoreID, c.UserID, id, procurement.Status(strings.TrimSpace(in.Status)))
		if err != nil {
			api.WriteError(w, api.Conflict("procurement_transition_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})))

	protected.Handle("POST /v1/network/procurements/{id}/cancel", auth.RequireRoles("owner", "admin", "warehouse")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_procurement_id", "procurement id must be a UUID"))
			return
		}
		out, err := procurementSvc.BuyerCancel(r.Context(), c.TenantID, c.StoreID, c.UserID, id)
		if err != nil {
			api.WriteError(w, api.Conflict("procurement_cancel_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})))

	protected.Handle("POST /v1/network/procurements/{id}/receive", auth.RequireRoles("owner", "admin", "warehouse")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_procurement_id", "procurement id must be a UUID"))
			return
		}
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			api.WriteError(w, api.BadRequest("missing_idempotency_key", "Idempotency-Key header is required"))
			return
		}
		out, err := procurementSvc.Receive(r.Context(), procurement.ReceiveCommand{BuyerTenantID: c.TenantID, BuyerStoreID: c.StoreID, ActorUserID: c.UserID, ProcurementID: id, IdempotencyKey: key})
		if err != nil {
			api.WriteError(w, api.Conflict("procurement_receive_rejected", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
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

	protected.Handle("POST /v1/edge/pairings", auth.RequireRoles("owner", "admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		var in struct {
			WarehouseID uuid.UUID `json:"warehouse_id"`
		}
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := edgeSvc.CreatePairing(r.Context(), c.TenantID, c.StoreID, in.WarehouseID, c.UserID)
		if err != nil {
			api.WriteError(w, api.Conflict("edge_pairing_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, out)
	})))
	protected.Handle("GET /v1/edge/devices", auth.RequireRoles("owner", "admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		items, err := edgeSvc.ListDevices(r.Context(), c.TenantID, c.StoreID)
		if err != nil {
			api.WriteError(w, api.Conflict("edge_devices_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})))
	protected.Handle("POST /v1/edge/devices/{id}/revoke", auth.RequireRoles("owner", "admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			api.WriteError(w, api.BadRequest("invalid_edge_device_id", "device id must be a UUID"))
			return
		}
		if err := edgeSvc.Revoke(r.Context(), c.TenantID, c.StoreID, id); err != nil {
			api.WriteError(w, api.Conflict("edge_revoke_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
	})))

	protected.Handle("GET /v1/audit-logs", auth.RequireRoles("owner", "admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.ClaimsFrom(r.Context())
		limit, offset, err := pageParams(r)
		if err != nil {
			api.WriteError(w, err)
			return
		}
		items, total, err := auditSvc.List(r.Context(), c.TenantID, c.StoreID, limit, offset)
		if err != nil {
			api.WriteError(w, api.Conflict("audit_query_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, pagedEnvelope(items, total, limit, offset))
	})))

	trustProxy := envBool("TRUST_PROXY_HEADERS", false)
	protectedHandler := auth.Middleware(verifier, audit.Middleware(pool, trustProxy, protected))
	root := http.NewServeMux()
	firstAllowedOrigin := func() string {
		for _, origin := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
			if v := strings.TrimSpace(origin); v != "" {
				return v
			}
		}
		return "http://localhost:3000"
	}
	edgeAuth := func(r *http.Request) (edge.Device, error) {
		return edgeSvc.Authenticate(r.Context(), r.Header.Get("X-Edge-Device-ID"), r.Header.Get("X-Edge-Secret"))
	}
	root.HandleFunc("POST /v1/edge/pair", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			PairCode   string `json:"pair_code"`
			DeviceName string `json:"device_name"`
		}
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		out, err := edgeSvc.PairDevice(r.Context(), in.PairCode, in.DeviceName)
		if err != nil {
			api.WriteError(w, api.BadRequest("edge_pair_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusCreated, map[string]any{"device_id": out.DeviceID, "device_secret": out.DeviceSecret, "store_id": out.StoreID, "warehouse_id": out.WarehouseID, "store_name": out.StoreName, "web_origin": firstAllowedOrigin()})
	})
	root.HandleFunc("GET /v1/edge/snapshot", func(w http.ResponseWriter, r *http.Request) {
		d, err := edgeAuth(r)
		if err != nil {
			api.WriteError(w, api.Unauthorized("edge_auth_failed", "edge device credentials are invalid"))
			return
		}
		out, err := edgeSvc.Snapshot(r.Context(), d)
		if err != nil {
			api.WriteError(w, api.Conflict("edge_snapshot_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, out)
	})
	root.HandleFunc("POST /v1/edge/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		if _, err := edgeAuth(r); err != nil {
			api.WriteError(w, api.Unauthorized("edge_auth_failed", "edge device credentials are invalid"))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	root.HandleFunc("POST /v1/edge/sales", func(w http.ResponseWriter, r *http.Request) {
		d, err := edgeAuth(r)
		if err != nil {
			api.WriteError(w, api.Unauthorized("edge_auth_failed", "edge device credentials are invalid"))
			return
		}
		var in struct {
			LocalOperationID string    `json:"local_operation_id"`
			OccurredAt       time.Time `json:"occurred_at"`
			PaymentMethod    string    `json:"payment_method"`
			Items            []struct {
				ProductID uuid.UUID `json:"product_id"`
				Qty       float64   `json:"qty"`
				UnitPrice int64     `json:"unit_price"`
				Title     string    `json:"title,omitempty"`
			} `json:"items"`
		}
		if err := decodeJSON(r, &in); err != nil {
			api.WriteError(w, err)
			return
		}
		localID := strings.TrimSpace(in.LocalOperationID)
		if localID == "" || len(localID) > 128 {
			api.WriteError(w, api.BadRequest("invalid_edge_local_id", "local_operation_id is required"))
			return
		}
		if in.PaymentMethod != "cash" && in.PaymentMethod != "card" {
			api.WriteError(w, api.BadRequest("offline_payment_not_supported", "offline sales support cash or card only"))
			return
		}
		items := make([]sales.CreateSaleItem, 0, len(in.Items))
		for _, x := range in.Items {
			items = append(items, sales.CreateSaleItem{ProductID: x.ProductID, Qty: x.Qty, UnitPrice: x.UnitPrice})
		}
		deviceID := d.ID
		occurred := in.OccurredAt
		if occurred.IsZero() {
			occurred = time.Now()
		}
		cmd := sales.CreateSaleCommand{TenantID: d.TenantID, StoreID: d.StoreID, WarehouseID: d.WarehouseID, PaymentMethod: in.PaymentMethod, IdempotencyKey: "edge:" + d.ID.String() + ":" + localID, Source: "edge", EdgeDeviceID: &deviceID, EdgeLocalOperationID: localID, EdgeOccurredAt: &occurred, Items: items}
		out, err := salesSvc.Create(r.Context(), cmd)
		if err != nil {
			_ = edgeSvc.RecordSync(r.Context(), d, localID, nil, "conflict", err.Error())
			api.WriteError(w, api.Conflict("edge_sale_conflict", err.Error()))
			return
		}
		_ = edgeSvc.RecordSync(r.Context(), d, localID, &out.ID, "synced", "")
		api.WriteJSON(w, http.StatusCreated, out)
	})
	root.HandleFunc("GET /v1/vehicles/catalog", func(w http.ResponseWriter, r *http.Request) {
		out, err := fitmentSvc.Catalog(r.Context())
		if err != nil {
			api.WriteError(w, api.Conflict("vehicle_catalog_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
	})
	publicSearchLimiter := httpx.NewRateLimiter(envInt("PUBLIC_SEARCH_RATE_LIMIT_PER_MINUTE", 120), time.Minute, trustProxy)
	root.Handle("GET /v1/network/search", publicSearchLimiter.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		var vehicleVariantID *uuid.UUID
		if raw := strings.TrimSpace(r.URL.Query().Get("vehicle_variant_id")); raw != "" {
			parsed, e := uuid.Parse(raw)
			if e != nil {
				api.WriteError(w, api.BadRequest("invalid_vehicle_variant_id", "vehicle_variant_id must be a UUID"))
				return
			}
			vehicleVariantID = &parsed
		}
		var vehicleYear *int
		if raw := strings.TrimSpace(r.URL.Query().Get("year")); raw != "" {
			y, e := strconv.Atoi(raw)
			if e != nil || y < 1200 || y > 2200 {
				api.WriteError(w, api.BadRequest("invalid_vehicle_year", "year must be between 1200 and 2200"))
				return
			}
			vehicleYear = &y
		}
		_, _ = reservationSvc.ExpireDue(r.Context(), 100)
		out, err := networkSvc.Search(r.Context(), q, vehicleVariantID, vehicleYear, lat, lng, r.URL.Query().Get("sort"), limit)
		if err != nil {
			api.WriteError(w, api.BadRequest("network_search_failed", err.Error()))
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"items": out, "count": len(out)})
	})))
	root.Handle("/v1/", protectedHandler)
	root.Handle("/healthz", public)
	root.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := timeout(r, 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			api.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "database": "down"})
			return
		}
		var applied bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, latestMigration).Scan(&applied); err != nil || !applied {
			api.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "database": "ok", "migration": latestMigration})
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"status": "ready", "database": "ok", "migration": latestMigration})
	})
	root.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		api.WriteJSON(w, http.StatusOK, map[string]string{"version": version, "commit": commit, "build_time": buildTime})
	})

	globalLimiter := httpx.NewRateLimiter(envInt("RATE_LIMIT_PER_MINUTE", 600), time.Minute, trustProxy)
	handler := httpx.RequestID(httpx.Recover(httpx.SecurityHeaders(envBool("ENABLE_HSTS", false), httpx.AccessLog(trustProxy, globalLimiter.Handler(cors(root))))))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Printf(`{"level":"info","event":"api_started","port":%q,"version":%q,"commit":%q}`, port, version, commit)

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-stopCtx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf(`{"level":"error","event":"api_shutdown_failed","error":%q}`, err.Error())
		} else {
			log.Print(`{"level":"info","event":"api_stopped"}`)
		}
	}
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

func pagedEnvelope(items any, total, limit, offset int) map[string]any {
	next := ""
	if offset+limit < total {
		next = pagination.EncodeOffset(offset + limit)
	}
	return map[string]any{"items": items, "total": total, "next_cursor": next}
}

func optionalUUIDQuery(r *http.Request, name string) (*uuid.UUID, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, api.BadRequest("invalid_"+name, name+" must be a UUID")
	}
	return &id, nil
}

func businessDateParam(r *http.Request, name string, fallback time.Time) (time.Time, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return time.Date(fallback.Year(), fallback.Month(), fallback.Day(), 0, 0, 0, 0, fallback.Location()), nil
	}
	v, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, api.BadRequest("invalid_"+name, name+" must use YYYY-MM-DD")
	}
	return v, nil
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

func businessDateRange(r *http.Request) (time.Time, time.Time, error) {
	now := time.Now()
	defaultFrom := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	defaultTo := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	parse := func(name string, fallback time.Time) (time.Time, error) {
		raw := strings.TrimSpace(r.URL.Query().Get(name))
		if raw == "" {
			return fallback, nil
		}
		v, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return time.Time{}, api.BadRequest("invalid_"+name, name+" must use YYYY-MM-DD")
		}
		return v, nil
	}
	from, err := parse("from", defaultFrom)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	to, err := parse("to", defaultTo)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, api.BadRequest("invalid_date_range", "to must not be before from")
	}
	return from, to, nil
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
		if origin = strings.TrimSpace(origin); origin != "" {
			set[origin] = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" && !set[origin] {
			api.WriteError(w, api.Forbidden("origin_not_allowed", "request origin is not allowed"))
			return
		}
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-ID")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, Retry-After")
			w.Header().Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func envInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func envBool(name string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}
