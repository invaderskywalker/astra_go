# Changelog

## 0.8.3 — 2026-08-23

- Added a workspace-access preflight for protected macOS directories.
- Added a retry/open-settings/quit recovery flow before starting a session.
- Prevented blocked workspaces from creating misleading agent runs.
- Added deterministic tests for filesystem access classification.
- Documented the permission boundary between Astra scopes and macOS privacy.

## 0.8.2 — 2026-08-23

- Added access-failure stopping logic after repeated operating-system denials.
- Moved runtime log rotation outside connected repositories by default.
- Improved durable session and run evidence.
