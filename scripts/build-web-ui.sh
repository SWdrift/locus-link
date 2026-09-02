#!/usr/bin/env bash
set -euo pipefail
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
pnpm --dir "$repo/internal/web/ui" install --frozen-lockfile
pnpm --dir "$repo/internal/web/ui" run type-check
pnpm --dir "$repo/internal/web/ui" run build
