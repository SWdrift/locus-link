#!/usr/bin/env bash
set -euo pipefail
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
"$repo/scripts/internal/build-go.sh" ./cmd/locus-backend "$repo/temp/bin/locus-backend" backend
