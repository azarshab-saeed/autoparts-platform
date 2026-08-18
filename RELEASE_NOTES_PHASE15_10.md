# Release notes — Phase 15.10

Checks pagination and acceptance hardening after the Phase 15.9.1 runtime hotfix.

The checks UI now follows the shared cursor pagination contract end-to-end: it requests at most 100 rows, displays the backend total, and appends additional pages with `next_cursor`. This prevents stores with more than 100 checks from seeing a silently truncated list. Outstanding-balance errors raised during check transitions are also normalized to the same Persian UX used by check creation.
