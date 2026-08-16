# Release Notes — Phase 4

Phase 4 changes the development cadence from backend-first to vertical product slices.

## Added
- `web/`: Next.js 16 / React 19.2 / TypeScript frontend
- Store RTL application shell and navigation
- Login screen with Mock and Real modes
- Store dashboard
- Operational new-sale screen
- API client for login, products, customers and sales
- Local session helper
- Mock catalog/customer/dashboard fixtures
- Responsive styling with no UI framework dependency
- Frontend container definition

## Run UI immediately (no backend)
```bash
cd web
cp .env.example .env.local
# keep NEXT_PUBLIC_MOCK_MODE=true
npm install
npm run dev
```
Open `http://localhost:3000/login`. In Mock Mode any email/password is accepted.

## Run against real Go backend
Set:
```env
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_MOCK_MODE=false
```
Bootstrap/login to the backend as documented in previous phases. Development CORS must allow `http://localhost:3000`.

## Known limitation in the generation environment
The container has Node/npm but external package download is unavailable, so `npm install` / `next build` could not be completed here. The source is structured for standard Next.js installation in a network-enabled environment.
