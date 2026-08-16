# Errors and Pagination

## Error envelope
Every application error is JSON:

```json
{
  "error": {
    "code": "validation_error",
    "message": "title is required",
    "details": {}
  }
}
```

Clients must branch on `error.code`, not localized message text.

## Pagination
List endpoints accept:
- `limit` from 1 to 100; default 30.
- `cursor` as an opaque value returned by the previous response.

Current Phase 2 cursor internally encodes an offset. It is intentionally treated as opaque so it can later move to keyset pagination without changing client contracts.
