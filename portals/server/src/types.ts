export interface User {
	username: string
	roles: string[]
	email: string
	display_name?: string
}

export interface Me {
	user: User
	actor: { capabilities?: Record<string, boolean> }
}

export interface ToolDefinition {
	id: string
	label: string
	description: string
	category: string
	path: string
	icon: string
	capabilities: string[]
	matchAll?: boolean
}

export interface FindResult {
	id: string
	label: string
	description: string
	path: string
	kind: 'tool' | 'account' | 'resource'
	score: number
}

export interface Account {
	id: string
	reseller_id?: string
	owner_user_id: string
	username: string
	primary_domain: string
	linux_uid: number
	linux_gid: number
	package_id: string
	status: string
	home_path: string
	ip_address?: string
	shell_class: string
	login_disabled: boolean
	desired_revision: number
	observed_revision: number
	usage?: Usage
}

export interface AccountDraft {
	username: string
	primaryDomain: string
	ownerEmail: string
	ownerPassword: string
	packageId: string
	resellerId: string
}

export interface AccountPayload {
	username: string
	primary_domain: string
	owner_email: string
	owner_password: string
	package_id: string
	reseller_id?: string
}

export interface FieldErrors {
	[field: string]: string
}

export interface Package {
	id: string
	reseller_id?: string
	name: string
	feature_set_id: string
	disk_bytes: number
	bandwidth_bytes_monthly: number
	domains: number
	subdomains: number
	alias_domains: number
	databases: number
	database_users: number
	mailboxes: number
	mailbox_storage_bytes: number
	ftp_users: number
	cron_jobs: number
	application_instances: number
	backup_retention_days: number
	cpu_percent: number
	memory_bytes: number
	process_limit: number
	io_weight: number
	iops: number
	concurrent_web_requests: number
	email_daily_limit: number
}

export interface Reseller {
	id: string
	user_id: string
	name: string
	brand_name?: string
	privilege_mask: string[]
	nameservers: string[]
	status: string
}

export interface Job {
	id: string
	type: string
	resource_type?: string
	resource_id?: string
	payload: Record<string, unknown>
	state: string
	priority: number
	attempts: number
	max_attempts: number
	progress: number
	run_after: string
	last_error?: string
	created_at: string
	started_at?: string
	finished_at?: string
	logs?: string[]
	retryable?: boolean
}

export interface AuditEvent {
	id: string
	occurred_at: string
	actor_type: string
	actor_id?: string
	effective_actor_id?: string
	account_id?: string
	action: string
	resource_type?: string
	resource_id?: string
	source_ip?: string
	request_id: string
	success: boolean
	before_state?: Record<string, unknown>
	after_state?: Record<string, unknown>
	metadata?: Record<string, unknown>
}

export interface Usage {
	account_id: string
	collected_at: string
	disk_bytes: number
	inode_count: number
	bandwidth_bytes: number
	cpu_percent: number
	memory_bytes: number
	process_count: number
}

export interface Service {
	name: string
	health: string
	desired_enabled: boolean
	observed_running: boolean
}

export interface ServerOverview {
	system: {
		hostname: string
		load1: number
		memory_used: number
		memory_total: number
		disk_used: number
		disk_total: number
		inodes_used: number
		inodes_total: number
		uptime_seconds: number
	}
	stats: { accounts: number; failedJobs: number }
	services: Service[]
}

export interface FeatureSet {
	id: string
	name: string
	features: Record<string, boolean>
}

export interface HostProcess {
	pid: number
	name: string
	user?: string
	command?: string
	scope?: string
}

export interface ResourceItem {
	id: string
	[key: string]: unknown
}

export interface PageResult<T> {
	items: T[]
	page: number
	pageSize: number
	pageCount: number
	total: number
}
