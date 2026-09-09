#!/usr/bin/env bash
# Copy .run/validation onto a live host at /var/lib/panel/validation.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck disable=SC1091
source "$ROOT/scripts/load-validation-env.sh"
DEST="${PANEL_VALIDATION_HOST_DIR:-/var/lib/panel/validation}"
sudo mkdir -p "$DEST"
sudo cp -a "$PANEL_VALIDATION_DIR/domain.env" "$DEST/domain.env"
sudo cp -a "$PANEL_VALIDATION_DIR/smtp.env" "$DEST/smtp.env"
sudo cp -a "$PANEL_VALIDATION_DIR/vm.env" "$DEST/vm.env"
if [[ -f "$PANEL_VALIDATION_DIR/smtp_password" ]]; then
	sudo cp -a "$PANEL_VALIDATION_DIR/smtp_password" "$DEST/smtp_password"
	sudo chmod 600 "$DEST/smtp_password"
fi
sudo chmod 640 "$DEST/"*.env
sudo chown -R root:panel "$DEST" 2>/dev/null || sudo chown -R root:root "$DEST"
echo "synced $PANEL_VALIDATION_DIR -> $DEST"
