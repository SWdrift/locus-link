#!/usr/bin/env bash
set -euo pipefail
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
"$repo/scripts/build-web-ui.sh"
"$repo/scripts/internal/build-go.sh" ./cmd/locus "$repo/temp/bin/locus" complete
