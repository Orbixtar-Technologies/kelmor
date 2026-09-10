interface NotificationBellProps {
	hasUnread?: boolean
}

export function NotificationBell ({ hasUnread = false }: NotificationBellProps) {
	return (
		<span className="notification-bell" aria-hidden="true">
			<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
				<path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9" />
				<path d="M13.73 21a2 2 0 0 1-3.46 0" />
			</svg>
			{hasUnread ? <span className="notification-dot" /> : null}
		</span>
	)
}
