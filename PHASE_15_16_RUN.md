# Phase 15.16 Runbook — Vehicle Notebook, QR Service History & Ownership

## Goal

Replace the paper notebook habit with a flow simple enough for a counter clerk, mechanic and vehicle owner:

**QR -> service / part / history -> save**

## Delivered

- store-scoped vehicle notebooks searchable by plate, VIN, owner name/phone and vehicle name;
- one stable QR token per vehicle;
- simple service/part entry with only title, mileage and date in the primary form;
- optional next mileage/date and notes behind “more details”;
- mechanic can open a QR and append verified service/part history without belonging to a store tenant;
- public owner page shows vehicle history without owner name, phone or VIN;
- owner can submit current mileage using a six-digit owner code;
- owner code is shown on notebook creation and can be rotated by store staff;
- owner-reported mileage is visibly marked separately from store/mechanic entries;
- next-service mileage/date is surfaced in both staff and owner views;
- QR generation uses the standard `qrcode` browser package;
- migration `021_vehicle_notebook.sql`.

## Apply

Use the Easy Apply package scripts. Default behavior applies files only. Optional `--commit` and `--push` are explicit.

Then run:

```bash
docker compose up -d --build
```

## Smoke acceptance

1. Open **دفتر خودرو** in the store panel.
2. Create a vehicle using plate + optional owner phone/name.
3. Confirm a QR and six-digit owner code are created.
4. Add **تعویض تسمه تایم**, mileage and date.
5. Add next due mileage and verify it appears in history.
6. Open the owner QR page and verify owner PII/VIN are not exposed.
7. Use the owner code once to submit a new mileage reading.
8. Log in as mechanic, open `/mechanic/vehicle-notebook?token=<QR_TOKEN>` and add a service entry.
9. Verify the mechanic entry appears on the owner page with provider attribution.
10. Rotate the owner code and verify the previous code no longer works.

## Product decision

Iranian Modian integration is intentionally deferred. Phase 15.16 is the vehicle notebook because it directly connects stores, mechanics and vehicle owners and solves a frequent real-world operational gap with minimal UI friction.
