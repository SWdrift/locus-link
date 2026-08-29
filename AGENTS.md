# AGENTS.md

## Workspace Safety

- All reads, writes, generated files, subprocess working directories, fixtures, and test state MUST remain inside this repository workspace.
- Tests MUST NOT read or modify user configuration, credentials, services, network state, or files outside the workspace.
- External-tool behavior MUST be exercised with deterministic helper executables created under `temp/`; tests MUST NOT depend on installed FRP/SSH/Salt services.
- Observation storage used by tests MUST be explicitly redirected to a path under the test workspace.
- End-to-end fixtures and binaries under `temp/e2e-run/` MUST remain after tests for manual reproduction; a new run MAY replace that directory deterministically.

## Verification

- Completion requires the workspace-local end-to-end CLI flow to pass.
- End-to-end fixtures MAY model projects, environments, and devices as directories under `temp/`.
- When a full-flow test exposes a Scope, registration, or model defect, update the implementation or design and rerun the complete flow.
- Perform at most three complete implementation/test iterations for the first vertical slice.


