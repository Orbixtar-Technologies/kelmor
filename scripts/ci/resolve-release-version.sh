#!/usr/bin/env bash
# Print the semver to use for PANEL_UPDATE_RELEASE / PANEL_DEB_VERSION.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MANIFEST="$ROOT/release-manifest.yaml"

if [[ -n "${PANEL_UPDATE_RELEASE:-}" ]]; then
	printf '%s\n' "$PANEL_UPDATE_RELEASE"
	exit 0
fi

if git -C "$ROOT" describe --tags --exact-match HEAD >/dev/null 2>&1; then
	git -C "$ROOT" describe --tags --exact-match HEAD | sed 's/^v//'
	exit 0
fi

base="$(grep -E '^platform_release:' "$MANIFEST" | awk '{print $2}')"
if [[ -z "$base" ]]; then
	echo "missing platform_release in $MANIFEST" >&2
	exit 1
fi
major="$(echo "$base" | cut -d. -f1)"
minor="$(echo "$base" | cut -d. -f2)"
patch="$(echo "$base" | cut -d. -f3)"
bump="${GITHUB_RUN_NUMBER:-$(git -C "$ROOT" rev-list --count HEAD)}"
printf '%s.%s.%s\n' "$major" "$minor" "$((patch + bump))"
