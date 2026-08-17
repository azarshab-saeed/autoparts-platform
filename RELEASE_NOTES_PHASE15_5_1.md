# Release Notes — Phase 15.5.1

## Scope

Targeted visual hotfix after reviewing the Phase 15.5 screenshots on Firefox/Linux at 1920×1080.

## Typography

The app no longer relies on Tahoma/Segoe UI being present on the client machine. The web build installs the Vazirmatn variable font package and the root layout imports its weight-axis stylesheet. The UI then uses `Vazirmatn Variable` first with Arabic-capable fallbacks.

Form controls explicitly inherit the same font. Numeric/OEM/SKU-heavy areas use tabular numerals and plaintext/isolate bidi behavior where appropriate.

## Density and contrast

The smallest operational text layer was enlarged, muted colors were strengthened, table row height increased, and panel metadata made easier to scan. Sidebar group labels, context information, and secondary helper text are no longer visually lost.

## Store workflow

The header no longer presents a decorative global-search input that does not execute a search. It now displays the current section and group. Sales and purchase empty states use a compact instructional card. Inventory and procurement receive readability and row-hierarchy improvements.

## Mechanic workflow

The search hero is shorter, filters remain visible but take less vertical space, and the results container is wider. Large desktop viewports use a three-column offer grid; medium desktop/tablet uses two columns; mobile stays one column. Card title, price, store, fitment, stock, and action text were increased selectively without making the card bulky.

## No business changes

No API, migrations, database writes, authentication semantics, inventory/accounting rules, reservation logic, or procurement logic changed.
