import { Link } from 'react-router-dom'
import { adminToolUrl } from '../admin-tool-url'
import { SecretValue } from '../components/ui'

interface DatabaseConnectionPanelProps {
	accountId: string
	credentials: Record<string, string> | null
	phpmyadminUrl?: string
	domain?: string
}

export function DatabaseConnectionPanel ({
	accountId,
	credentials,
	phpmyadminUrl,
	domain,
}: DatabaseConnectionPanelProps) {
	const phpmyadminHref = adminToolUrl('phpmyadmin', phpmyadminUrl, domain)
	return (
		<section className="panel">
			<h2>Connection details</h2>
			<p className="subtle">Database users are provisioned automatically. Passwords stay hidden until you reveal or copy them. Use phpMyAdmin or a SQL client with these settings.</p>
			{credentials ? <dl className="detail-list">
				<div><dt>Host</dt><dd>{credentials.host || '127.0.0.1'}</dd></div>
				<div><dt>Username</dt><dd><code>{credentials.username}</code></dd></div>
				<div><dt>Password</dt><dd><SecretValue value={credentials.password || ''} /></dd></div>
			</dl> : <p className="subtle">Credentials appear after the first database is provisioned.</p>}
			<div className="admin-links">
				{phpmyadminHref ? <a href={phpmyadminHref} target="_blank" rel="noopener noreferrer">Open phpMyAdmin</a> : null}
				<Link to={`/sql?account=${accountId}`}>Database Manager</Link>
				<Link to={`/files?account=${accountId}`}>File manager</Link>
			</div>
		</section>
	)
}
