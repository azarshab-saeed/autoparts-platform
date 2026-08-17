package audit

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/example/autoparts-core/internal/platform/auth"
	"github.com/example/autoparts-core/internal/platform/httpx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Entry struct {
	ID          int64     `json:"id"`
	OccurredAt  time.Time `json:"occurred_at"`
	RequestID   string    `json:"request_id"`
	ActorUserID uuid.UUID `json:"actor_user_id"`
	Role        string    `json:"role"`
	Method      string    `json:"method"`
	Path        string    `json:"path"`
	Route       string    `json:"route"`
	Status      int       `json:"status"`
	RemoteIP    string    `json:"remote_ip"`
	Metadata    any       `json:"metadata,omitempty"`
}

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func (s *Service) List(ctx context.Context, tenantID, storeID uuid.UUID, limit, offset int) ([]Entry, int, error) {
	var total int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE tenant_id=$1 AND store_id=$2`, tenantID, storeID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(ctx, `
      SELECT id,occurred_at,request_id,actor_user_id,role,method,path,route,status,remote_ip,metadata
      FROM audit_logs WHERE tenant_id=$1 AND store_id=$2 ORDER BY occurred_at DESC,id DESC LIMIT $3 OFFSET $4`, tenantID, storeID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Entry, 0)
	for rows.Next() {
		var e Entry
		var metadata []byte
		if err := rows.Scan(&e.ID, &e.OccurredAt, &e.RequestID, &e.ActorUserID, &e.Role, &e.Method, &e.Path, &e.Route, &e.Status, &e.RemoteIP, &metadata); err != nil {
			return nil, 0, err
		}
		if len(metadata) > 0 {
			var v any
			if json.Unmarshal(metadata, &v) == nil {
				e.Metadata = v
			}
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func Middleware(db *pgxpool.Pool, trustProxy bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isMutation(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		sw := &auditStatusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		claims, ok := auth.ClaimsFrom(r.Context())
		if !ok || claims.TenantID == uuid.Nil || claims.StoreID == uuid.Nil {
			return
		}
		route := r.Pattern
		if route == "" {
			route = r.Method + " " + r.URL.Path
		}
		meta, _ := json.Marshal(map[string]any{"user_agent": strings.TrimSpace(r.UserAgent())})
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, err := db.Exec(ctx, `
          INSERT INTO audit_logs(tenant_id,store_id,actor_user_id,role,request_id,method,path,route,status,remote_ip,metadata)
          VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			claims.TenantID, claims.StoreID, claims.UserID, claims.Role, httpx.RequestIDFrom(r.Context()), r.Method, r.URL.Path, route, sw.Status(), httpx.ClientIP(r, trustProxy), meta)
		if err != nil {
			log.Printf(`{"level":"error","event":"audit_write_failed","request_id":%q}`, httpx.RequestIDFrom(r.Context()))
		}
	})
}

func isMutation(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

type auditStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w *auditStatusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
		w.ResponseWriter.WriteHeader(code)
	}
}
func (w *auditStatusWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}
func (w *auditStatusWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}
