export const SUPPORTED_PHP_VERSIONS = ['8.3', '8.4', '8.5'] as const

export type SupportedPHPVersion = (typeof SUPPORTED_PHP_VERSIONS)[number]

export function isSupportedPHPVersion (version: string) {
	return (SUPPORTED_PHP_VERSIONS as readonly string[]).includes(version)
}
