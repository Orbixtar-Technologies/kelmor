import { Link } from 'react-router-dom'
import { listOf } from '../client'
import { useLoad } from '../lib/hooks'
import { DataTable, type Column } from '../components/data-table'
import { EmptyState, Notice, PageHeader, Panel, Pill, StatusPill } from '../components/ui'
import { formatBytes } from '../lib/format'
import type { AccountRow } from '../components/account-picker'

interface MailDomainRow { id: string; account_id: string; domain_id: string; catchall_policy: string; status: string }
interface MailboxRow { id: string; account_id: string; local_part: string; domain_id: string; quota_bytes: number; status: string }
interface DomainRow { id: string; ascii_fqdn: string }

interface DomainView extends MailDomainRow { username: string; fqdn: string; mailboxes: number }
interface MailboxView extends MailboxRow { username: string; address: string }

export function MailOverview () {
	const accounts = useLoad<AccountRow[]>(() => listOf<AccountRow>('/api/v1/accounts'), [])
	const list = accounts.data ?? []
	const key = list.map((a) => a.id).join(',')

	const mail = useLoad<{ domains: DomainView[]; mailboxes: MailboxView[] }>(async () => {
		const perAccount = await Promise.all(
			list.map(async (account) => {
				const [mailDomains, mailboxes, domains] = await Promise.all([
					listOf<MailDomainRow>(`/api/v1/accounts/${account.id}/mail/domains`).catch(() => []),
					listOf<MailboxRow>(`/api/v1/accounts/${account.id}/mail/mailboxes`).catch(() => []),
					listOf<DomainRow>(`/api/v1/accounts/${account.id}/domains`).catch(() => []),
				])
				const fqdnOf = (domainId: string) => domains.find((d) => d.id === domainId)?.ascii_fqdn ?? account.primary_domain
				return {
					domains: mailDomains.map((domain) => ({
						...domain,
						username: account.username,
						fqdn: fqdnOf(domain.domain_id),
						mailboxes: mailboxes.filter((m) => m.domain_id === domain.domain_id).length,
					})),
					mailboxes: mailboxes.map((mailbox) => ({
						...mailbox,
						username: account.username,
						address: `${mailbox.local_part}@${fqdnOf(mailbox.domain_id)}`,
					})),
				}
			}),
		)
		return {
			domains: perAccount.flatMap((p) => p.domains),
			mailboxes: perAccount.flatMap((p) => p.mailboxes),
		}
	}, [key])

	const domains = mail.data?.domains ?? []
	const mailboxes = mail.data?.mailboxes ?? []

	const domainColumns: Column<DomainView>[] = [
		{ key: 'fqdn', header: 'Mail domain', sort: (d) => d.fqdn, render: (d) => <span className="mono"><strong>{d.fqdn}</strong></span> },
		{ key: 'account', header: 'Account', sort: (d) => d.username, render: (d) => <Link to={`/accounts/${d.account_id}?tab=email`}>{d.username}</Link> },
		{
			key: 'catchall',
			header: 'Catch-all',
			sort: (d) => d.catchall_policy,
			render: (d) => <Pill tone={d.catchall_policy === 'reject' ? 'ok' : 'warn'}>{d.catchall_policy}</Pill>,
		},
		{ key: 'mailboxes', header: 'Mailboxes', align: 'right', sort: (d) => d.mailboxes },
		{ key: 'status', header: 'Status', sort: (d) => d.status, render: (d) => <StatusPill status={d.status} /> },
	]

	const mailboxColumns: Column<MailboxView>[] = [
		{ key: 'address', header: 'Mailbox', sort: (m) => m.address, render: (m) => <span className="mono">{m.address}</span> },
		{ key: 'account', header: 'Account', sort: (m) => m.username, render: (m) => <Link to={`/accounts/${m.account_id}?tab=email`}>{m.username}</Link> },
		{ key: 'quota', header: 'Quota', align: 'right', sort: (m) => m.quota_bytes, render: (m) => (m.quota_bytes ? formatBytes(m.quota_bytes) : 'package default') },
		{ key: 'status', header: 'Status', sort: (m) => m.status, render: (m) => <StatusPill status={m.status} /> },
	]

	return (
		<>
			<PageHeader
				title="Mail Domains and Mailboxes"
				description="Mail routing across the whole server: which domains Kelmor accepts mail for, their catch-all policy and every mailbox behind them."
				favoritePath="/email"
			/>
			<Notice tone="info">
				Each mail domain gets a 2048-bit DKIM key and a <span className="mono">default._domainkey</span> TXT record when it
				is created, signed through rspamd with the Postfix milter. A catch-all of <span className="mono">reject</span> is
				the safe default: <span className="mono">discard</span> hides delivery failures from senders.
			</Notice>

			<Panel title="Mail domains" icon="mail" subtitle={`${domains.length} routed`} tight>
				<DataTable
					rows={domains}
					columns={domainColumns}
					rowKey={(d) => d.id}
					loading={accounts.loading || mail.loading}
					error={mail.error}
					searchPlaceholder="Search domain or account"
					noun="mail domains"
					initialSort={{ key: 'fqdn', dir: 'asc' }}
					empty={
						<EmptyState icon="mail" title="No mail domain is routed yet">
							A mail domain is created with each account primary domain during provisioning.
						</EmptyState>
					}
				/>
			</Panel>

			<Panel title="Mailboxes" icon="inbox" subtitle={`${mailboxes.length} mailboxes`} tight>
				<DataTable
					rows={mailboxes}
					columns={mailboxColumns}
					rowKey={(m) => m.id}
					loading={accounts.loading || mail.loading}
					error={mail.error}
					searchPlaceholder="Search address or account"
					noun="mailboxes"
					initialSort={{ key: 'address', dir: 'asc' }}
					empty={
						<EmptyState icon="inbox" title="No mailbox exists yet" action={<Link className="btn secondary" to="/accounts">Open List Accounts</Link>}>
							Customers create mailboxes in Kelmor Control; an operator can create one from the account summary.
						</EmptyState>
					}
				/>
			</Panel>
		</>
	)
}
