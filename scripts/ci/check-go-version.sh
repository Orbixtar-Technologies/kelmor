#!/usr/bin/env bash
set -euo pipefail

ROOT="${KELMOR_ROOT:-$(cd "$(dirname "$0")/../.." && pwd)}"

manifest_version="$(
	awk '$1 == "go_baseline:" {gsub(/"/, "", $2); print $2}' \
		"$ROOT/release-manifest.yaml"
)"
module_version="$(awk '$1 == "go" {print $2}' "$ROOT/go.mod")"

if [[ -z "$manifest_version" || -z "$module_version" ]]; then
	echo "unable to read Go versions from go.mod and release-manifest.yaml" >&2
	exit 1
fi

module_baseline="${module_version%.*}"
if [[ "$module_baseline" != "$manifest_version" ]]; then
	echo "Go version mismatch: go.mod=$module_version release-manifest.yaml=$manifest_version" >&2
	exit 1
fi

for workflow in ci.yml release.yml; do
	workflow_version="$(
		awk '$1 == "go-version:" {gsub(/"/, "", $2); print $2}' \
			"$ROOT/.github/workflows/$workflow"
	)"
	if [[ "$workflow_version" != "$manifest_version" ]]; then
		echo "Go version mismatch: $workflow=$workflow_version release-manifest.yaml=$manifest_version" >&2
		exit 1
	fi
done

echo "Go baseline consistent at $manifest_version"
