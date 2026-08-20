# Release Notes — Phase 15.16

## Vehicle Notebook & QR Service History

- Added a simple digital vehicle notebook for stores and mechanics.
- Added stable per-vehicle QR access.
- Added service, consumed-part and mileage history.
- Added optional next-service mileage/date reminders.
- Added an owner-facing history page with private owner details hidden.
- Added owner mileage reporting protected by a six-digit owner code.
- Added separate labels for owner-reported mileage versus store/mechanic history.
- Added mechanic QR workflow without requiring mechanic tenant/store membership.

Migration: `021_vehicle_notebook.sql`.
