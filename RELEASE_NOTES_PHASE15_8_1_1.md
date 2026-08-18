# Phase 15.8.1.1 Release Notes

- Added OS/architecture detection and platform-specific Store Agent installer selection to `/store/edge`.
- Added automatic Manager polling after installer download, plus a manual reconnect probe.
- Added stable GitHub Release installer aliases so production web builds do not need a version-specific URL.
- Added Linux Debian/Ubuntu packages for amd64 and arm64 with a globally enabled user-level systemd Manager service.
- Linux runtime binaries live in the user's data directory so signed self-update does not require root.
- Added combined Windows/Linux installer release workflow and GitHub Release publication for `store-agent-v*` tags.
- Added release-configured Manager CORS origins for first-install detection before Pairing.
- Store Agent build version advanced to `0.15.8.2`.
