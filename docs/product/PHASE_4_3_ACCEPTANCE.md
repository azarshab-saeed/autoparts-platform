# Phase 4.3 Acceptance - Keycloak

Phase 4.3 is accepted when the following works in a local Docker environment:

1. `docker compose up --build` starts PostgreSQL, db-prepare, Keycloak, API, and Web.
2. Keycloak Admin Console is reachable at `http://localhost:8081/admin`.
3. Realm `autoparts` exists.
4. Client `autoparts-web` exists as a public OIDC client with standard flow enabled.
5. Realm roles include owner/admin/cashier/warehouse/accountant/mechanic/consumer.
6. Login from `http://localhost:3000/login` redirects to Keycloak.
7. `owner@example.com / ChangeMe123!` authenticates successfully.
8. Returned access token contains `tenant_id`, `store_id`, realm role `owner`, and audience `autoparts-api`.
9. `/v1/me` returns the seeded store `یدکی رضایی` and default warehouse.
10. Requests without bearer tokens receive HTTP 401.
11. Tokens with wrong issuer/audience/signature are rejected.
12. Store/Sales pages no longer read authentication tokens from localStorage.
13. Logout ends the Keycloak browser session and returns to `/login`.

## Security invariants
- User password is handled by Keycloak, not the Go API or Next.js application form.
- Tenant/store context comes from verified token claims only.
- Business authorization remains enforced by Go role guards and store-scoped queries.
- Access-token signature verification uses Keycloak JWKS and an exact public issuer.
