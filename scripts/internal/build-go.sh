#!/usr/bin/env bash
set -euo pipefail
target=${1:?target required}
output=${2:?output required}
artifact=${3:?artifact required}
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
version="$(tr -d '\r\n' < "$repo/VERSION")"
[[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "invalid VERSION: $version" >&2; exit 2; }
commit=$(git -C "$repo" rev-parse HEAD 2>/dev/null || printf unknown)
if [[ -n $(git -C "$repo" status --porcelain --untracked-files=normal 2>/dev/null || true) ]]; then commit="${commit}-dirty"; fi
mkdir -p "$(dirname "$output")"
cd "$repo"
go build -ldflags "-X locus-link/internal/buildinfo.Version=$version -X locus-link/internal/buildinfo.Commit=$commit -X locus-link/internal/buildinfo.Artifact=$artifact" -o "$output" "$target"
printf 'Built %s\n' "$output"
