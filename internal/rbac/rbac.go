package rbac

import "strings"

const (
	ServerRead            = "server.read"
	ServerSettingsWrite   = "server.settings.write"
	ServerServicesRead    = "server.services.read"
	ServerServicesRestart = "server.services.restart"
	ServerFirewallRead    = "server.firewall.read"
	ServerFirewallWrite   = "server.firewall.write"
	AccountsRead          = "accounts.read"
	AccountsCreate        = "accounts.create"
	AccountsModify        = "accounts.modify"
	AccountsSuspend       = "accounts.suspend"
	AccountsTerminate     = "accounts.terminate"
	AccountsImpersonate   = "accounts.impersonate"
	ResellersRead         = "resellers.read"
	ResellersCreate       = "resellers.create"
	ResellersModify       = "resellers.modify"
	PackagesRead          = "packages.read"
	PackagesWrite         = "packages.write"
	DomainsRead           = "domains.read"
	DomainsWrite          = "domains.write"
	DNSRead               = "dns.read"
	DNSWrite              = "dns.write"
	WebsitesRead          = "websites.read"
	WebsitesWrite         = "websites.write"
	ApplicationsRead      = "applications.read"
	ApplicationsWrite     = "applications.write"
	DatabasesRead         = "databases.read"
	DatabasesWrite        = "databases.write"
	MailRead              = "mail.read"
	MailWrite             = "mail.write"
	BackupsRead           = "backups.read"
	BackupsCreate         = "backups.create"
	BackupsRestore        = "backups.restore"
	SecurityAuditRead     = "security.audit.read"
	APITokensRead         = "api_tokens.read"
	APITokensWrite        = "api_tokens.write"
	FilesRead             = "files.read"
	FilesWrite            = "files.write"
	CronRead              = "cron.read"
	CronWrite             = "cron.write"
	BillingUsageRead      = "billing.usage.read"
)

var All = []string{
	ServerRead, ServerSettingsWrite, ServerServicesRead, ServerServicesRestart,
	ServerFirewallRead, ServerFirewallWrite,
	AccountsRead, AccountsCreate, AccountsModify, AccountsSuspend, AccountsTerminate, AccountsImpersonate,
	ResellersRead, ResellersCreate, ResellersModify,
	PackagesRead, PackagesWrite,
	DomainsRead, DomainsWrite, DNSRead, DNSWrite,
	WebsitesRead, WebsitesWrite, ApplicationsRead, ApplicationsWrite,
	DatabasesRead, DatabasesWrite, MailRead, MailWrite,
	BackupsRead, BackupsCreate, BackupsRestore,
	SecurityAuditRead, APITokensRead, APITokensWrite,
	FilesRead, FilesWrite, CronRead, CronWrite, BillingUsageRead,
}

var RoleCaps = map[string][]string{
	"root_owner":             All,
	"server_administrator":   All,
	"server_operator":        {ServerRead, ServerServicesRead, ServerServicesRestart, AccountsRead, PackagesRead, DomainsRead, DNSRead, WebsitesRead, SecurityAuditRead, BillingUsageRead},
	"reseller":               {AccountsRead, AccountsCreate, AccountsModify, AccountsSuspend, PackagesRead, PackagesWrite, DomainsRead, DomainsWrite, DNSRead, WebsitesRead, BackupsRead, BackupsCreate, BackupsRestore, BillingUsageRead},
	"customer_owner":         {DomainsRead, DomainsWrite, DNSRead, DNSWrite, WebsitesRead, WebsitesWrite, ApplicationsRead, ApplicationsWrite, DatabasesRead, DatabasesWrite, MailRead, MailWrite, BackupsRead, BackupsCreate, BackupsRestore, FilesRead, FilesWrite, CronRead, CronWrite, APITokensRead, APITokensWrite, BillingUsageRead},
	"customer_administrator": {DomainsRead, DomainsWrite, DNSRead, DNSWrite, WebsitesRead, WebsitesWrite, ApplicationsRead, ApplicationsWrite, DatabasesRead, DatabasesWrite, MailRead, MailWrite, FilesRead, FilesWrite, CronRead, CronWrite},
	"customer_user":          {DomainsRead, WebsitesRead, MailRead, FilesRead, DatabasesRead, DNSRead},
	"auditor":                {ServerRead, AccountsRead, SecurityAuditRead, BillingUsageRead},
}

type Actor struct {
	UserID         string          `json:"user_id"`
	Username       string          `json:"username"`
	Roles          []string        `json:"roles"`
	Capabilities   map[string]bool `json:"capabilities"`
	AccountIDs     []string        `json:"account_ids"`
	ResellerID     string          `json:"reseller_id,omitempty"`
	ImpersonatorID string          `json:"impersonator_id,omitempty"`
	IsServerScope  bool            `json:"is_server_scope"`
}

func (a Actor) Has(cap string) bool {
	if a.Capabilities[cap] {
		return true
	}
	for _, r := range a.Roles {
		for _, c := range RoleCaps[r] {
			if c == cap {
				return true
			}
		}
	}
	return false
}

func (a Actor) CanAccount(accountID string) bool {
	if a.IsServerScope {
		return true
	}
	for _, id := range a.AccountIDs {
		if id == accountID {
			return true
		}
	}
	return false
}

func Expand(roles []string, extra []string) map[string]bool {
	out := map[string]bool{}
	for _, r := range roles {
		for _, c := range RoleCaps[r] {
			out[c] = true
		}
	}
	for _, c := range extra {
		c = strings.TrimSpace(c)
		if c != "" {
			out[c] = true
		}
	}
	return out
}
