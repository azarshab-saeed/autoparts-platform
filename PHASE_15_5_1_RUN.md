# Phase 15.5.1 — Persian Typography & Visual Density Fix

This hotfix is designed to be applied **after Phase 15.5 UI Polish**.
It does not change the Go API, database schema, migrations, Keycloak behavior, accounting, reservations, or procurement business logic.

## What changed

- Self-hosted-at-build Persian UI font through `@fontsource-variable/vazirmatn`.
- All controls inherit the same Persian font, including Firefox/Linux form controls.
- Larger and higher-contrast secondary text, tables, badges, sidebar labels, and operational metadata.
- Better mixed RTL/LTR rendering for SKU/OEM/numeric content.
- The non-functional topbar search is replaced by an honest current-page context indicator.
- Sales and purchases get compact instructional empty states instead of large blank panels.
- Inventory table rows and KPI summaries are easier to scan.
- Network/procurement secondary text and actions are clearer.
- Mechanic hero is shorter after the visual polish, results use more desktop width, and 1920px-class screens show three result cards per row.
- Mobile keeps a single-column mechanic result list and bottom store navigation.

## Apply

```bash
unzip AutoParts_Phase15_5_1_Persian_Typography_EasyApply.zip
cd AutoParts_Phase15_5_1_Persian_Typography_EasyApply
./apply.sh --repo ../autoparts-core/
```

Because `web/package.json` changes, rebuild the web image:

```bash
cd ../autoparts-core/
docker compose up -d --build web
```

If Docker has an unusually sticky npm layer, force only the web rebuild:

```bash
docker compose build --no-cache web
docker compose up -d web
```

## Visual smoke checklist

Check these at normal browser zoom and once at mobile width:

1. `/store` — Persian letters should look balanced, KPI/table text should no longer be tiny.
2. `/store/sales` — empty invoice state should be compact and readable.
3. `/store/purchases` — same empty-state/readability check.
4. `/store/inventory` — row spacing, low-stock row visibility, numeric columns.
5. `/store/procurement` — seller/buyer metadata and status chips.
6. `/mechanic` — compact hero and three cards per row on a 1920px desktop viewport.
7. Mixed Persian + OEM/SKU strings should remain visually stable in Firefox.

## Optional commit / push

```bash
./apply.sh --repo ../autoparts-core/ --commit
# or
./apply.sh --repo ../autoparts-core/ --push
```

Default commit message:

```text
style: improve Persian typography and visual density
```

The script prints the exact revert command and stores it in `LAST_COMMIT.txt` if it creates a commit.
