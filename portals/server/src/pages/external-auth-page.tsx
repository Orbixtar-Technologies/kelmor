import { featureById } from '../whm-catalog'
import { WhmToolBody } from './whm-tool-page'

export function ExternalAuthPage () {
	return (
		<>
			<p className="subtle">
				Save queues a host apply that writes <code>/etc/panel/external-auth.json</code>.
				A local Director account is still required. When LDAP is required, login also binds with the operator password.
				Browser OIDC redirect is not implemented — issuer and client ID are host-applied configuration only.
			</p>
			<WhmToolBody feature={featureById('external-auth')!} />
		</>
	)
}
