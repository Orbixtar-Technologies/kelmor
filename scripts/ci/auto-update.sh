#!/usr/bin/env bash
# One-shot: build signed release and publish update feed to configured targets.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
bash "$ROOT/scripts/ci/release.sh"
bash "$ROOT/scripts/ci/publish-update-feed.sh"
echo "Auto-update pipeline complete."
