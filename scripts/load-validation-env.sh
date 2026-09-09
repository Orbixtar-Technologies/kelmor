# shellcheck shell=bash
# Source from the repo root or any cwd. Sets PANEL_VALIDATION_DIR and exports
# keys from domain.env, smtp.env, and vm.env. Resolves relative secret paths.
if [[ -n "${PANEL_VALIDATION_LOADED:-}" ]]; then
	return 0 2>/dev/null || exit 0
fi

_panel_find_validation() {
	if [[ -n "${PANEL_VALIDATION_DIR:-}" && -d "$PANEL_VALIDATION_DIR" ]]; then
		printf '%s' "$PANEL_VALIDATION_DIR"
		return 0
	fi
	if [[ -d /var/lib/panel/validation ]]; then
		printf '%s' /var/lib/panel/validation
		return 0
	fi
	local here start
	here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
	if [[ -d "$here/.run/validation" ]]; then
		printf '%s' "$here/.run/validation"
		return 0
	fi
	start="$PWD"
	while [[ "$start" != / ]]; do
		if [[ -d "$start/.run/validation" ]]; then
			printf '%s' "$start/.run/validation"
			return 0
		fi
		start="$(dirname "$start")"
	done
	return 1
}

_panel_export_envfile() {
	local file="$1"
	[[ -f "$file" ]] || return 0
	local line key val
	while IFS= read -r line || [[ -n "$line" ]]; do
		line="${line%%#*}"
		line="${line#"${line%%[![:space:]]*}"}"
		line="${line%"${line##*[![:space:]]}"}"
		[[ -z "$line" ]] && continue
		key="${line%%=*}"
		val="${line#*=}"
		[[ "$key" == "$line" ]] && continue
		export "$key=$val"
	done < "$file"
}

PANEL_VALIDATION_DIR="$(_panel_find_validation)" || {
	echo "load-validation-env: no .run/validation or /var/lib/panel/validation" >&2
	return 1 2>/dev/null || exit 1
}
export PANEL_VALIDATION_DIR

_panel_export_envfile "$PANEL_VALIDATION_DIR/domain.env"
_panel_export_envfile "$PANEL_VALIDATION_DIR/smtp.env"
_panel_export_envfile "$PANEL_VALIDATION_DIR/vm.env"

if [[ -n "${SMTP_SECRET_PATH:-}" && "${SMTP_SECRET_PATH}" != /* ]]; then
	export SMTP_SECRET_PATH="$PANEL_VALIDATION_DIR/${SMTP_SECRET_PATH}"
fi
if [[ -n "${VM_KEY_PATH:-}" && "${VM_KEY_PATH}" != /* ]]; then
	export VM_KEY_PATH="$PANEL_VALIDATION_DIR/${VM_KEY_PATH}"
fi
if [[ -z "${PANEL_PUBLIC_IPV4:-}" && -n "${VM_PUBLIC_IPV4:-}" ]]; then
	export PANEL_PUBLIC_IPV4="$VM_PUBLIC_IPV4"
fi
if [[ -z "${PANEL_HOSTNAME:-}" && -n "${TEST_DOMAIN:-}" ]]; then
	export PANEL_HOSTNAME="$TEST_DOMAIN"
fi
if [[ -f "${SMTP_SECRET_PATH:-}" ]]; then
	SMTP_PASSWORD="$(tr -d '\r\n' < "$SMTP_SECRET_PATH")"
	export SMTP_PASSWORD
fi

export PANEL_NS1_HOSTNAME="${NS1_HOSTNAME:-}"
export PANEL_NS2_HOSTNAME="${NS2_HOSTNAME:-}"
export PANEL_VALIDATION_LOADED=1
unset -f _panel_find_validation _panel_export_envfile
