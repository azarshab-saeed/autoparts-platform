# AutoParts 0.15.0-rc.1 Operator Checklist

## Automated gate

```bash
make rc-check
```

Capture the output of `/version`, smoke check count, database invariant result and load-test p95.

## Browser regression

Use separate browser profiles/tabs for `owner@example.com`, `pars@example.com` and `mechanic@example.com`.

- Owner: dashboard, inventory, sale, purchase, accounting/report pages.
- Mechanic: OEM `4254.97` search, vehicle-fitment filter, reservation create/cancel.
- Pars owner: incoming reservation accept/ready; verify tenant isolation.
- Owner + Pars: procurement request -> accepted -> ready -> received; verify purchase/sale and balances on both stores.
- Return flow: sale return and purchase return on suitable test documents.
- Daily close: expected cash/card and duplicate-close protection.
- Audit: mutations appear with request ID, actor, role and HTTP status.

## Release evidence

Record:

- Git commit SHA.
- `0.15.0-rc.1` image/version output.
- PostgreSQL backup path before staging deploy.
- Keycloak backup path before staging deploy.
- `make rc-check` output.
- One screenshot of each critical browser flow.

## Promotion rule

Promote RC1 only when the automated gate is fully green and there are no Severity 1/2 defects. Severity 3 defects require an explicit documented waiver. Any auth, tenant-isolation, inventory-hold, accounting-balance or data-loss defect blocks promotion.
