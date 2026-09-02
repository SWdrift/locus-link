#!/usr/bin/env bash
set -euo pipefail
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo"
"$repo/scripts/build-web-ui.sh"
go test ./test -run EndToEnd -count=1
printf 'E2E artifacts retained at %s\n' "$repo/temp/e2e-run"
