#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

mkdir -p "$TMP/.github/workflows"
cp "$ROOT/go.mod" "$TMP/go.mod"
cp "$ROOT/release-manifest.yaml" "$TMP/release-manifest.yaml"
cp "$ROOT/.github/workflows/ci.yml" "$TMP/.github/workflows/ci.yml"
cp "$ROOT/.github/workflows/release.yml" "$TMP/.github/workflows/release.yml"

check() {
	KELMOR_ROOT="$TMP" bash "$ROOT/scripts/ci/check-go-version.sh"
}

check >/dev/null

for file in go.mod release-manifest.yaml .github/workflows/ci.yml .github/workflows/release.yml; do
	cp "$ROOT/$file" "$TMP/$file"
	case "$file" in
		go.mod)
			sed -i 's/^go .*/go 0.0.0/' "$TMP/$file"
			;;
		release-manifest.yaml)
			sed -i 's/^go_baseline:.*/go_baseline: "0.0"/' "$TMP/$file"
			;;
		.github/workflows/*)
			sed -i 's/go-version: .*/go-version: "0.0"/' "$TMP/$file"
			;;
	esac
	if check >/dev/null 2>&1; then
		echo "expected version mismatch for $file" >&2
		exit 1
	fi
	cp "$ROOT/$file" "$TMP/$file"
done

echo "Go version consistency validation ok"
