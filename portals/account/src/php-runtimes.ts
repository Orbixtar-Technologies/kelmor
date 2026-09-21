export const SUPPORTED_PHP_VERSIONS = ['8.3', '8.4', '8.5'] as const

export interface PHPRuntime {
	version: string
	status: string
}

export function isSupportedPHPVersion (version: string) {
	return (SUPPORTED_PHP_VERSIONS as readonly string[]).includes(version)
}

export function installedPHPVersions (runtimes: readonly PHPRuntime[]) {
	return runtimes
		.filter((runtime) => runtime.status === 'installed' && isSupportedPHPVersion(runtime.version))
		.map((runtime) => runtime.version)
}

export function isInstalledPHPVersion (version: string, installed: readonly string[]) {
	return installed.includes(version)
}

export function missingHostPHPVersion (recorded: string, installed: readonly string[]) {
	if (!recorded || isInstalledPHPVersion(recorded, installed)) return ''
	return recorded
}
