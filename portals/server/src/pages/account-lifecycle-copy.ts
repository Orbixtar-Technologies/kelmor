export type AccountLifecycleAction = 'create' | 'modify' | 'suspend' | 'unsuspend' | 'terminate'

export function accountHomePath (username: string): string {
	return `/home/${username}`
}

export function accountDatabasePrefix (username: string): string {
	return `${username}_`
}

export function provisionPipelineSteps (username: string, domain: string): string[] {
	const home = accountHomePath(username)
	return [
		`Create Linux user ${username} with a dedicated UID/GID and home ${home}.`,
		`Seed ${home}/public_html, ${home}/public_ftp, mail, logs, backups, and SSH metadata.`,
		`Register the account, ${domain} ownership, package limits, and an audit event in Kelmor desired state.`,
		`Bind an nginx vhost and PHP-FPM pool to ${username} so web processes run as that UID.`,
		`Publish a DNS zone for ${domain} with A, www, mail, MX, SPF, and DMARC records.`,
		`Enable mail routing for ${domain} and create info@${domain}.`,
		`Create ${username}_db with user ${username}_u so databases stay prefixed to this tenant.`,
	]
}

export function isolationLines (account: {
	username: string
	linux_uid: number
	linux_gid: number
	home_path: string
	shell_class: string
}): string[] {
	const jail = account.shell_class === 'sftp-only'
		? 'SFTP is jailed to the home directory (Kelmor chroot, not cPanel VirtFS).'
		: `Shell class ${account.shell_class}.`
	return [
		`POSIX identity UID ${account.linux_uid} / GID ${account.linux_gid} owns ${account.home_path}.`,
		'Home is 0751 root:root with a tenant ACL so SFTP can chroot and neighboring accounts cannot list files.',
		jail,
		`Web and PHP-FPM workers run as ${account.username}, confined to that home tree.`,
		`Databases and users must use the ${accountDatabasePrefix(account.username)} prefix.`,
	]
}

export function lifecycleImpact (action: AccountLifecycleAction): string[] {
	switch (action) {
		case 'create':
			return [
				'Allocates a Linux user, home, DNS zone, vhost, mail, and a prefixed database.',
				'The account stays provisioning until the durable job finishes.',
			]
		case 'modify':
			return [
				'Updates package, ownership, domain, IP, or login flags without wiping site files.',
				'Queues a full account reconcile so quota, features, and service bindings follow the new assignment.',
			]
		case 'suspend':
			return [
				'Locks the Linux login and freezes the account cgroup.',
				'nginx serves a 503 holding page for the account vhosts.',
				'Cron and inbound mail for this tenant stop until unsuspend.',
			]
		case 'unsuspend':
			return [
				'Unlocks the Linux user and thaws the cgroup.',
				'Restores live vhosts, cron, and mail routing.',
			]
		case 'terminate':
			return [
				'Drops prefixed databases, DNS zones, vhosts, PHP pools, mail trees, and certificates.',
				'Removes the home directory and the Linux user.',
				'This cannot be undone from Director.',
			]
		default: {
			const exhaustive: never = action
			return exhaustive
		}
	}
}

export function bandwidthEnforcementCopy (): string {
	return 'When monthly bandwidth is exceeded, Kelmor holds HTTP with 509. It does not auto-suspend the account.'
}

export function quotaEnforcementCopy (): string {
	return 'Disk limits use setquota when the filesystem supports usrquota, plus a panel quota file. Over-quota SFTP becomes read-only.'
}
