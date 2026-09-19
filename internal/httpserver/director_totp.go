package httpserver

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/auth"
	"github.com/hosting-panel/panel/internal/hostconfig"
	"github.com/hosting-panel/panel/internal/pkg/secret"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
)

func (a *API) totpBox() (*secret.Box, error) {
	return secret.LoadOrCreate(filepath.Join(a.panelStateDir(), "secrets", "master.key"))
}

func (a *API) requireServerOperator(w http.ResponseWriter, r *http.Request) *store.User {
	if !a.require(w, r, rbac.ServerRead) {
		return nil
	}
	if !actor(r).IsServerScope {
		a.fail(w, r, 403, "FORBIDDEN", "Server scope required", false)
		return nil
	}
	user := a.Store.UserByID(actor(r).UserID)
	if user == nil {
		a.fail(w, r, 401, "UNAUTHENTICATED", "Sign in again", false)
		return nil
	}
	return user
}

func (a *API) userTOTPValid(user *store.User, code string) bool {
	if user == nil || len(user.TOTPSecretEnc) == 0 {
		return false
	}
	box, err := a.totpBox()
	if err != nil {
		return false
	}
	plain, err := box.Decrypt(user.TOTPSecretEnc)
	if err != nil {
		return false
	}
	return auth.VerifyTOTP(string(plain), strings.TrimSpace(code))
}

func ldapUserDN(template, username string) string {
	dn := strings.TrimSpace(template)
	if dn == "" {
		dn = "uid={username},ou=people,dc=example,dc=com"
	}
	dn = strings.ReplaceAll(dn, "{username}", username)
	return strings.ReplaceAll(dn, "%s", username)
}

func (a *API) loginFactorExtras(r *http.Request, user *store.User, password, totpCode string) (map[string]any, string, string) {
	extras := map[string]any{}
	settings := a.settingsFile()
	provider := hostconfig.ExternalAuthProvider(settings)
	if hostconfig.ExternalAuthRequired(settings) {
		switch provider {
		case "ldap":
			if err := a.verifyLDAPLogin(r, settings, user.Username, password); err != nil {
				return nil, "EXTERNAL_AUTH_FAILED", "LDAP bind failed for this operator"
			}
		case "oidc":
			extras["oidc_sso"] = "not_implemented"
		}
	}
	if user.TOTPEnabled {
		if strings.TrimSpace(totpCode) == "" {
			return nil, "TOTP_REQUIRED", "Enter the authenticator code for this operator"
		}
		if !a.userTOTPValid(user, totpCode) {
			return nil, "INVALID_TOTP", "Invalid authenticator code"
		}
	}
	if hostconfig.TwoFactorRequired(settings) && !user.TOTPEnabled {
		extras["totp_enroll_required"] = true
	}
	return extras, "", ""
}

func (a *API) verifyLDAPLogin(r *http.Request, settings hostconfig.File, username, password string) error {
	url := strings.TrimSpace(hostconfig.Field(settings, "external_auth", "ldap_url", ""))
	if url == "" {
		return nil
	}
	dn := ldapUserDN(hostconfig.Field(settings, "external_auth", "ldap_user_dn", ""), username)
	_, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "VerifyLDAPBind",
		Params: mustJSON(map[string]any{"url": url, "user_dn": dn, "password": password}),
	})
	return err
}

func (a *API) enrollTOTP(w http.ResponseWriter, r *http.Request) {
	user := a.requireServerOperator(w, r)
	if user == nil {
		return
	}
	if user.TOTPEnabled {
		a.fail(w, r, 409, "TOTP_ALREADY_ENABLED", "Two-factor is already enrolled for this operator", false)
		return
	}
	secretPlain, url, err := auth.NewTOTP("Kelmor Director", user.Username)
	if err != nil {
		a.fail(w, r, 500, "TOTP_ERROR", "Could not generate authenticator secret", false)
		return
	}
	box, err := a.totpBox()
	if err != nil {
		a.fail(w, r, 500, "TOTP_ERROR", "Could not open secret store", false)
		return
	}
	enc, err := box.Encrypt([]byte(secretPlain))
	if err != nil {
		a.fail(w, r, 500, "TOTP_ERROR", "Could not store authenticator secret", false)
		return
	}
	user.TOTPSecretEnc = enc
	user.TOTPEnabled = false
	a.Store.PutUser(user)
	a.audit(r, "auth.totp.enroll", "user", user.ID, true, nil, map[string]any{"pending": true})
	writeJSON(w, 200, map[string]any{"otpauth_url": url, "secret": secretPlain, "pending": true})
}

func (a *API) confirmTOTP(w http.ResponseWriter, r *http.Request) {
	user := a.requireServerOperator(w, r)
	if user == nil {
		return
	}
	var in struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid confirmation", false)
		return
	}
	if user.TOTPEnabled {
		a.fail(w, r, 409, "TOTP_ALREADY_ENABLED", "Two-factor is already enrolled for this operator", false)
		return
	}
	if !a.userTOTPValid(user, in.Code) {
		a.fail(w, r, 400, "INVALID_TOTP", "Invalid authenticator code", false)
		return
	}
	user.TOTPEnabled = true
	a.Store.PutUser(user)
	a.audit(r, "auth.totp.confirm", "user", user.ID, true, nil, map[string]any{"totp_enabled": true})
	writeJSON(w, 200, map[string]any{"ok": true, "totp_enabled": true})
}

func (a *API) disableTOTP(w http.ResponseWriter, r *http.Request) {
	user := a.requireServerOperator(w, r)
	if user == nil {
		return
	}
	var in struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if user.TOTPEnabled && !a.userTOTPValid(user, in.Code) {
		a.fail(w, r, 400, "INVALID_TOTP", "Invalid authenticator code", false)
		return
	}
	user.TOTPEnabled = false
	user.TOTPSecretEnc = nil
	a.Store.PutUser(user)
	a.audit(r, "auth.totp.disable", "user", user.ID, true, map[string]any{"totp_enabled": true}, map[string]any{"totp_enabled": false})
	writeJSON(w, 200, map[string]any{"ok": true, "totp_enabled": false})
}
