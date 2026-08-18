# Phase 15.8.1 Release Notes — Store Agent Lifecycle Manager

- Added a persistent local Store Agent Manager on `127.0.0.1:17623`.
- Store Edge worker is now supervised as a child process and automatically restarted after unexpected exits.
- `/store/edge` can Start, Stop and Restart the worker on Windows and Linux without Terminal/systemctl/Services UI.
- Stop keeps the Manager alive, so Start remains available from the frontend.
- Added signed update check/apply flow with SHA-256 + Ed25519 verification.
- Added worker-only hot update and Manager+Worker service update helper.
- Windows installer now installs Manager + Worker + Updater and migrates away from the legacy direct worker service.
- Linux installer now installs `autoparts-store-edge-manager.service` and migrates existing local data when appropriate.
- Production cloud origin learned during pairing is also trusted by the local Manager CORS guard.
- Added manager API spec and release manifest signing tool.
