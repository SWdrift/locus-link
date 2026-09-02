#!/usr/bin/env bash
set -euo pipefail
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo"
"$repo/scripts/build-web-ui.sh"
go test ./...
pnpm --dir .tools/markdown install --frozen-lockfile
pnpm --dir .tools/markdown run check:links
printf 'Verification passed. E2E artifacts retained at %s\n' "$repo/temp/e2e-run"
