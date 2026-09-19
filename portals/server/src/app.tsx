import { useEffect, useState } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { api, clearToken, getToken } from './client'
import { DirectorShell } from './layout/director-shell'
import { AccountFunctionPage } from './pages/account-function-page'
import { AccountServicesPage } from './pages/account-services-page'
import { AccountSummaryPage } from './pages/account-summary-page'
import { AccountsPage } from './pages/accounts-page'
import { AuditPage } from './pages/audit-page'
import { CreateAccountPage } from './pages/create-account-page'
import { CronPage } from './pages/cron-page'
import { DeliverabilityPage } from './pages/deliverability-page'
import { DNSPage } from './pages/dns-page'
import { DnsCleanupPage, DnsSynchronizePage } from './pages/dns-inventory-tools-page'
import { DomainsPage } from './pages/domains-page'
import { EmailManagerPage } from './pages/email-manager-page'
import { IPUsagePage } from './pages/ip-usage-page'
import { MailDeliveryReportsPage } from './pages/mail-delivery-reports-page'
import { FeatureManagerPage } from './pages/feature-manager-page'
import { FileManagerPage } from './pages/file-manager-page'
import { FTPPage } from './pages/ftp-page'
import { HomePage } from './pages/home-page'
import { ProcessManagerPage } from './pages/process-manager-page'
import { SQLManagerPage } from './pages/sql-manager-page'
import { SSLManagerPage } from './pages/ssl-manager-page'
import { WebmailPage } from './pages/webmail-page'
import { JobsPage } from './pages/jobs-page'
import { PackagesPage } from './pages/packages-page'
import { ExternalAuthPage } from './pages/external-auth-page'
import { InitialQuotaPage } from './pages/initial-quota-page'
import { LinkNodesPage } from './pages/link-nodes-page'
import { LoginPage } from './pages/login-page'
import { ResetBandwidthPage } from './pages/reset-bandwidth-page'
import { ResellerUsagePage } from './pages/reseller-usage-page'
import { ResellersPage } from './pages/resellers-page'
import { SkeletonDirectoryPage } from './pages/skeleton-directory-page'
import { TwoFactorPage } from './pages/two-factor-page'
import { SecurityPage } from './pages/security-page'
import { SecurityToolPage } from './pages/security-tool-page'
import { SqlToolPage } from './pages/sql-tool-page'
import { ServiceStatusPage } from './pages/service-status-page'
import { TrackDeliveryPage } from './pages/track-delivery-page'
import { TransfersPage } from './pages/transfers-page'
import { UpdateToolPage } from './pages/update-tool-page'
import { UpdatesPage } from './pages/updates-page'
import { UsagePage } from './pages/usage-page'
import { WebsitesPage } from './pages/websites-page'
import { HubPage, ToolRedirect } from './pages/hub-page'
import { CapProvider, Forbidden, hasCapabilities } from './rbac'
import type { ReactNode } from 'react'
import type { Me } from './types'

export function App () {
	const [me, setMe] = useState<Me | null>(null)
	const [checking, setChecking] = useState(Boolean(getToken()))
	useEffect(() => {
		if (!getToken()) return
		api<Me>('/api/v1/me').then(setMe).catch(() => clearToken()).finally(() => setChecking(false))
	}, [])
	if (checking) return <main className="auth"><div className="loading-state" role="status"><span />Restoring Kelmor Director session…</div></main>
	if (!me) return <LoginPage onLogin={setMe} />
	const capabilities = me.actor.capabilities || {}
	function allowed (requiredCapabilities: string | readonly string[], element: ReactNode) {
		const required = typeof requiredCapabilities === 'string' ? [requiredCapabilities] : requiredCapabilities
		return hasCapabilities(capabilities, required) ? element : <Forbidden title="Access restricted" />
	}
	return (
		<CapProvider caps={capabilities}>
			<Routes>
				<Route element={<DirectorShell me={me} onSignOut={() => { api('/api/v1/auth/logout', { method: 'POST', body: '{}' }).catch(() => undefined); clearToken(); setMe(null) }} />}>
					<Route index element={(capabilities['server.read'] || capabilities['accounts.read']) ? <HomePage /> : <Forbidden title="Home" />} />
					<Route path="accounts" element={allowed('accounts.read', <AccountsPage />)} />
					<Route path="accounts/create" element={allowed(['accounts.create', 'packages.read'], <CreateAccountPage />)} />
					<Route path="accounts/ownership" element={allowed('accounts.modify', <AccountFunctionPage toolId="change-ownership" />)} />
					<Route path="accounts/modify" element={allowed(['accounts.read', 'accounts.modify'], <AccountFunctionPage toolId="modify-account" />)} />
					<Route path="accounts/change-package" element={allowed(['accounts.read', 'accounts.modify'], <AccountFunctionPage toolId="change-package" />)} />
					<Route path="accounts/force-password" element={allowed(['accounts.read', 'accounts.modify'], <AccountFunctionPage toolId="force-password" />)} />
					<Route path="accounts/password" element={allowed('accounts.modify', <AccountFunctionPage toolId="password-modification" />)} />
					<Route path="accounts/suspension" element={allowed(['accounts.read', 'accounts.suspend'], <AccountFunctionPage toolId="suspend-account" />)} />
					<Route path="accounts/terminate" element={allowed(['accounts.read', 'accounts.terminate'], <AccountFunctionPage toolId="terminate-account" />)} />
					<Route path="accounts/remove" element={allowed(['accounts.read', 'accounts.terminate'], <AccountFunctionPage toolId="remove-terminated-account" />)} />
					<Route path="accounts/login-control" element={allowed(['accounts.read', 'accounts.impersonate'], <AccountFunctionPage toolId="login-control" />)} />
					<Route path="accounts/summary" element={allowed('accounts.read', <AccountFunctionPage toolId="account-summary" />)} />
					<Route path="accounts/tokens" element={allowed('api_tokens.read', <AccountFunctionPage toolId="api-tokens-whm" />)} />
					<Route path="accounts/reset-bandwidth" element={allowed('accounts.modify', <ResetBandwidthPage />)} />
					<Route path="accounts/skeleton" element={allowed('server.read', <SkeletonDirectoryPage />)} />
					<Route path="accounts/:id" element={allowed('accounts.read', <AccountSummaryPage />)} />
					<Route path="accounts/:id/services" element={allowed('accounts.read', <AccountServicesPage />)} />
					<Route path="packages" element={allowed('packages.read', <PackagesPage />)} />
					<Route path="packages/add" element={allowed('packages.write', <PackagesPage focus="add" />)} />
					<Route path="packages/delete" element={allowed('packages.write', <PackagesPage focus="delete" />)} />
					<Route path="features" element={allowed('packages.read', <FeatureManagerPage />)} />
					<Route path="resellers" element={allowed('resellers.read', <ResellersPage />)} />
					<Route path="resellers/usage" element={allowed('resellers.read', <ResellerUsagePage />)} />
					<Route path="domains" element={allowed(['accounts.read', 'domains.read'], <DomainsPage />)} />
					<Route path="domains/add" element={allowed('domains.write', <DomainsPage focus="add" />)} />
					<Route path="domains/delete" element={allowed('domains.write', <DomainsPage focus="delete" />)} />
					<Route path="domains/park" element={allowed('domains.write', <DomainsPage focus="park" />)} />
					<Route path="websites" element={allowed(['accounts.read', 'websites.read'], <WebsitesPage />)} />
					<Route path="dns" element={allowed('dns.read', <DNSPage />)} />
					<Route path="dns/edit" element={allowed('dns.write', <DNSPage />)} />
					<Route path="dns/cleanup" element={allowed('dns.read', <DnsCleanupPage />)} />
					<Route path="dns/synchronize" element={allowed('dns.read', <DnsSynchronizePage />)} />
					<Route path="files" element={allowed(['accounts.read', 'files.read'], <FileManagerPage />)} />
					<Route path="ftp" element={allowed(['accounts.read', 'files.read'], <FTPPage />)} />
					<Route path="cron" element={allowed(['accounts.read', 'cron.read'], <CronPage />)} />
					<Route path="sql" element={allowed(['accounts.read', 'databases.read'], <SQLManagerPage />)} />
					<Route path="sql/password" element={allowed('databases.write', <SqlToolPage toolId="password" />)} />
					<Route path="sql/processes" element={allowed('databases.read', <SqlToolPage toolId="processes" />)} />
					<Route path="email" element={allowed(['accounts.read', 'mail.read'], <EmailManagerPage />)} />
					<Route path="mail/delivery-reports" element={allowed(['mail.read', 'accounts.read'], <MailDeliveryReportsPage />)} />
					<Route path="mail/track-delivery" element={allowed('mail.read', <TrackDeliveryPage />)} />
					<Route path="ip-usage" element={allowed('accounts.read', <IPUsagePage />)} />
					<Route path="deliverability" element={allowed(['accounts.read', 'mail.read', 'dns.read'], <DeliverabilityPage />)} />
					<Route path="webmail" element={allowed(['accounts.read', 'mail.read'], <WebmailPage />)} />
					<Route path="webmail/client" element={allowed('mail.read', <WebmailPage />)} />
					<Route path="ssl" element={allowed(['accounts.read', 'websites.read'], <SSLManagerPage />)} />
					<Route path="ssl/request" element={allowed('websites.write', <SSLManagerPage />)} />
					<Route path="ssl/install" element={allowed('websites.write', <SSLManagerPage />)} />
					<Route path="ssl/autossl" element={allowed('websites.write', <SSLManagerPage />)} />
					<Route path="ssl/inventory" element={allowed('websites.read', <SSLManagerPage />)} />
					<Route path="ssl/status" element={allowed('websites.read', <SSLManagerPage />)} />
					<Route path="ssl/service" element={allowed('websites.read', <SSLManagerPage />)} />
					<Route path="processes" element={allowed('server.read', <ProcessManagerPage />)} />
					<Route path="processes/daily" element={allowed('server.read', <ProcessManagerPage />)} />
					<Route path="server/link-nodes" element={allowed('server.settings.write', <LinkNodesPage />)} />
					<Route path="server/initial-quota" element={allowed('server.read', <InitialQuotaPage />)} />
					<Route path="status" element={allowed('server.read', <ServiceStatusPage />)} />
					<Route path="status/info" element={allowed('server.read', <ServiceStatusPage focus="info" />)} />
					<Route path="status/services" element={allowed('server.read', <ServiceStatusPage focus="services" />)} />
					<Route path="status/http" element={allowed('server.read', <ServiceStatusPage focus="http" />)} />
					<Route path="security" element={allowed('server.read', <SecurityPage />)} />
					<Route path="security/external-auth" element={allowed('server.settings.write', <ExternalAuthPage />)} />
					<Route path="security/two-factor" element={allowed('server.settings.write', <TwoFactorPage />)} />
					<Route path="security/advisor" element={allowed('server.read', <SecurityToolPage toolId="advisor" />)} />
					<Route path="security/reboot" element={allowed('server.read', <SecurityToolPage toolId="reboot" />)} />
					<Route path="security/force-reboot" element={allowed('server.read', <SecurityToolPage toolId="force-reboot" />)} />
					<Route path="transfers" element={allowed('accounts.read', <TransfersPage />)} />
					<Route path="transfers/restore" element={allowed('accounts.read', <TransfersPage focus="restore" />)} />
					<Route path="transfers/copy" element={allowed('accounts.create', <TransfersPage focus="copy" />)} />
					<Route path="import" element={allowed('accounts.read', <TransfersPage />)} />
					<Route path="jobs" element={(capabilities['server.read'] || capabilities['accounts.read']) ? <JobsPage /> : <Forbidden title="Jobs" />} />
					<Route path="jobs/queue" element={(capabilities['server.read'] || capabilities['accounts.read']) ? <JobsPage /> : <Forbidden title="Jobs" />} />
					<Route path="updates" element={allowed('server.read', <UpdatesPage />)} />
					<Route path="updates/preferences" element={allowed('server.read', <UpdateToolPage toolId="preferences" />)} />
					<Route path="updates/changelog" element={allowed('server.read', <UpdateToolPage toolId="changelog" />)} />
					<Route path="audit" element={allowed('security.audit.read', <AuditPage />)} />
					<Route path="usage" element={allowed(['billing.usage.read', 'accounts.read', 'packages.read'], <UsagePage />)} />
					<Route path="usage/disk" element={allowed(['billing.usage.read', 'accounts.read', 'packages.read'], <UsagePage />)} />
					<Route path="monitor" element={allowed(['billing.usage.read', 'accounts.read', 'packages.read'], <UsagePage />)} />
					<Route path="section/:hubId" element={<HubPage />} />
					<Route path="tools/:toolId" element={<ToolRedirect />} />
					<Route path="*" element={<Navigate to="/" replace />} />
				</Route>
			</Routes>
		</CapProvider>
	)
}

