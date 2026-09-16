export interface RequestToken {
	scope: string
	sequence: number
}

export class RequestSequence {
	private sequence = 0
	private readonly currentByScope = new Map<string, number>()

	begin (scope: string): RequestToken {
		const token = { scope, sequence: ++this.sequence }
		this.currentByScope.set(scope, token.sequence)
		return token
	}

	invalidate (scope: string) {
		this.currentByScope.set(scope, ++this.sequence)
	}

	isCurrent (token: RequestToken) {
		return this.currentByScope.get(token.scope) === token.sequence
	}
}
