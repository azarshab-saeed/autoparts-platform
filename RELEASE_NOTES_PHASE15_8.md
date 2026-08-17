# Release Notes — Phase 15.8

- Store Edge version `0.15.8`.
- Local Hardware Bridge configuration persisted atomically in `hardware.json`.
- Receipt adapters: Windows Unicode spool, raw TCP 9100, Windows raw share, file QA.
- Receipt auto-print after online/offline sale; cash sale can auto-open configured drawer.
- ESC/POS cash-drawer pulse support.
- Barcode scanner keyboard-wedge Enter flow in `/store/sales`.
- ZPL/text label printing bridge.
- A4 PDF system-spool endpoint.
- POS provider abstraction: manual, explicitly gated QA mock, generic TCP JSON adapter.
- `/store/hardware` guided local configuration and hardware test UI.
- Private-network/CORS loopback preflight support for browser-to-Agent calls.
- Separate `edge/openapi.yaml` documents the local loopback API.
- No new database migration; cloud latest remains `014_store_edge_offline.sql`.

- Rebased on Phase 15.7.1 + 15.7.1.1: production Store Agent installer/service lifecycle and strict same-origin loopback pairing fix are preserved.
- QA file transport is explicitly gated by `AUTOPARTS_EDGE_ALLOW_FILE_TRANSPORT=true`; production compose never enables it.
- Hardware config validates TCP host:port, Windows UNC share safety, default A4 printer semantics, POS timeout bounds, and cash-drawer/raw-printer compatibility.
