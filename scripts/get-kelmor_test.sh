#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

mkdir -p "$TMP/bin" "$TMP/web" "$TMP/payload/kelmor-installer_0.0.0-test_linux_amd64/bin" \
	"$TMP/payload/kelmor-installer_0.0.0-test_linux_amd64/share/portals/server" \
	"$TMP/payload/kelmor-installer_0.0.0-test_linux_amd64/share/portals/account" \
	"$TMP/host/etc"

printf '#!/bin/sh\nprintf "panel-install %%s\\n" "$*"\n' \
	>"$TMP/payload/kelmor-installer_0.0.0-test_linux_amd64/bin/panel-install"
chmod 0755 "$TMP/payload/kelmor-installer_0.0.0-test_linux_amd64/bin/panel-install"
printf '<!doctype html><title>Kelmor Director</title>\n' \
	>"$TMP/payload/kelmor-installer_0.0.0-test_linux_amd64/share/portals/server/index.html"
printf '<!doctype html><title>Kelmor Control</title>\n' \
	>"$TMP/payload/kelmor-installer_0.0.0-test_linux_amd64/share/portals/account/index.html"
install -m 0755 "$ROOT/installer/bundle/install.sh" \
	"$TMP/payload/kelmor-installer_0.0.0-test_linux_amd64/install.sh"

tar -C "$TMP/payload" -czf "$TMP/web/kelmor-installer-linux-amd64.tar.gz" \
	kelmor-installer_0.0.0-test_linux_amd64
(cd "$TMP/web" && sha256sum kelmor-installer-linux-amd64.tar.gz >kelmor-installer-linux-amd64.tar.gz.sha256)

cat >"$TMP/bin/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
out=""
url=""
while [[ $# -gt 0 ]]; do
	case "$1" in
	-o)
		out="$2"
		shift 2
		;;
	-fsSL | -f | -s | -S | -L)
		shift
		;;
	*)
		url="$1"
		shift
		;;
	esac
done
echo "$url" >>"${KELMOR_CURL_LOG}"
base="$(basename "$url")"
if [[ ! -f "${KELMOR_WEB}/$base" ]]; then
	echo "curl mock: missing $base" >&2
	exit 22
fi
cp "${KELMOR_WEB}/$base" "$out"
EOF
chmod 0755 "$TMP/bin/curl"

export PATH="$TMP/bin:$PATH"
export KELMOR_WEB="$TMP/web"
export KELMOR_CURL_LOG="$TMP/curl.log"
export KELMOR_DOWNLOAD_BASE="https://github.com/OrbixtarTechnologies/public/releases/latest/download"
: >"$KELMOR_CURL_LOG"

printf 'NAME="Debian GNU/Linux"\nVERSION_ID="12"\n' >"$TMP/host/etc/os-release"
if bash "$ROOT/scripts/get-kelmor.sh" --root "$TMP/host" >/dev/null 2>&1; then
	echo "expected Debian prefix to fail" >&2
	exit 1
fi

printf 'NAME="Ubuntu"\nVERSION_ID="24.04"\n' >"$TMP/host/etc/os-release"
out="$(bash "$ROOT/scripts/get-kelmor.sh" --root "$TMP/host" --hostname panel.example.net --admin-email ops@example.net)"
[[ "$out" == *"panel-install"* ]] || {
	echo "expected panel-install to run: $out" >&2
	exit 1
}
[[ "$out" == *"--hostname panel.example.net"* ]] || {
	echo "hostname not forwarded: $out" >&2
	exit 1
}
grep -Fq "${KELMOR_DOWNLOAD_BASE}/kelmor-installer-linux-amd64.tar.gz" "$KELMOR_CURL_LOG" || {
	echo "did not fetch GitHub latest installer: $(cat "$KELMOR_CURL_LOG")" >&2
	exit 1
}
grep -Fq "${KELMOR_DOWNLOAD_BASE}/kelmor-installer-linux-amd64.tar.gz.sha256" "$KELMOR_CURL_LOG" || {
	echo "did not fetch installer checksum" >&2
	exit 1
}

grep -Fq 'uses: softprops/action-gh-release@v2' "$ROOT/.github/workflows/release.yml" || {
	echo "release workflow must publish a GitHub Release" >&2
	exit 1
}
grep -Fq 'dist/installer/get-kelmor.sh' "$ROOT/.github/workflows/release.yml" || {
	echo "release workflow must attach get-kelmor.sh" >&2
	exit 1
}
grep -A8 '^[[:space:]]*publish_feed:' "$ROOT/.github/workflows/release.yml" | grep -q 'default: false' || {
	echo "VM feed publish must stay default false" >&2
	exit 1
}

echo GET_KELMOR_OK
