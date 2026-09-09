/* Inline stroke icons. Kept local so Director ships no icon-font dependency
   and every tool tile, nav row and status pill draws from the same set. */

export type IconName = keyof typeof paths

export interface IconProps {
	name: IconName
	size?: number
	className?: string
	title?: string
}

export function Icon ({ name, size = 16, className, title }: IconProps) {
	return (
		<svg
			className={className}
			width={size}
			height={size}
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth={1.85}
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden={title ? undefined : true}
			role={title ? 'img' : undefined}
			focusable="false"
		>
			{title ? <title>{title}</title> : null}
			{paths[name]}
		</svg>
	)
}

const paths = {
	home: <><path d="M3 10.5 12 3l9 7.5" /><path d="M5 9.5V21h14V9.5" /></>,
	users: <><circle cx="9" cy="8" r="3.2" /><path d="M2.5 20a6.5 6.5 0 0 1 13 0" /><path d="M16.5 5.3a3.2 3.2 0 0 1 0 6.2" /><path d="M18 14.4a6.5 6.5 0 0 1 3.5 5.6" /></>,
	userPlus: <><circle cx="9" cy="8" r="3.2" /><path d="M2.5 20a6.5 6.5 0 0 1 13 0" /><path d="M18.5 8v6M21.5 11h-6" /></>,
	userCheck: <><circle cx="9" cy="8" r="3.2" /><path d="M2.5 20a6.5 6.5 0 0 1 13 0" /><path d="m16 11.5 2 2 4-4" /></>,
	userX: <><circle cx="9" cy="8" r="3.2" /><path d="M2.5 20a6.5 6.5 0 0 1 13 0" /><path d="m16.5 8.5 5 5M21.5 8.5l-5 5" /></>,
	userMinus: <><circle cx="9" cy="8" r="3.2" /><path d="M2.5 20a6.5 6.5 0 0 1 13 0" /><path d="M21.5 11h-6" /></>,
	key: <><circle cx="7.5" cy="15.5" r="3.5" /><path d="m10 13 8.5-8.5" /><path d="m16 7 2.5 2.5M19 4l2 2" /></>,
	box: <><path d="M21 8 12 3 3 8v8l9 5 9-5Z" /><path d="m3 8 9 5 9-5M12 13v8" /></>,
	layers: <><path d="m12 3 9 5-9 5-9-5Z" /><path d="m3 13 9 5 9-5" /></>,
	briefcase: <><rect x="3" y="7.5" width="18" height="12" rx="2" /><path d="M9 7.5V6a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v1.5M3 12.5h18" /></>,
	globe: <><circle cx="12" cy="12" r="9" /><path d="M3 12h18" /><path d="M12 3a15 15 0 0 1 0 18a15 15 0 0 1 0-18Z" /></>,
	activity: <path d="M2.5 12H7l3-7.5 4 15 3-7.5h4.5" />,
	cpu: <><rect x="6" y="6" width="12" height="12" rx="1.6" /><path d="M9.5 9.5h5v5h-5z" /><path d="M9 3v3M15 3v3M9 18v3M15 18v3M3 9h3M3 15h3M18 9h3M18 15h3" /></>,
	shield: <><path d="M12 3 4.5 6v6c0 4.4 3 7.9 7.5 9 4.5-1.1 7.5-4.6 7.5-9V6Z" /><path d="m9 12 2 2 4-4" /></>,
	power: <><path d="M12 3v9" /><path d="M18 6.3a8 8 0 1 1-12 0" /></>,
	archive: <><rect x="3" y="4" width="18" height="4.5" rx="1" /><path d="M5 8.5V20h14V8.5M10 12.5h4" /></>,
	upload: <><path d="M12 16V4" /><path d="m7.5 8.5 4.5-4.5 4.5 4.5" /><path d="M4 16v3.5h16V16" /></>,
	download: <><path d="M12 4v12" /><path d="m7.5 11.5 4.5 4.5 4.5-4.5" /><path d="M4 16v3.5h16V16" /></>,
	list: <><path d="M8.5 6.5h12M8.5 12h12M8.5 17.5h12" /><path d="M4 6.5h.01M4 12h.01M4 17.5h.01" /></>,
	fileText: <><path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8Z" /><path d="M14 3v5h5M9 13h6M9 17h4" /></>,
	database: <><ellipse cx="12" cy="6" rx="8" ry="3" /><path d="M4 6v12c0 1.7 3.6 3 8 3s8-1.3 8-3V6" /><path d="M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3" /></>,
	mail: <><rect x="3" y="5" width="18" height="14" rx="2" /><path d="m3.5 7 8.5 6 8.5-6" /></>,
	lock: <><rect x="4.5" y="10.5" width="15" height="10" rx="2" /><path d="M8 10.5V7.5a4 4 0 0 1 8 0v3" /></>,
	barChart: <><path d="M4 20V11M10 20V4M16 20v-6M22 20H2" /></>,
	search: <><circle cx="10.5" cy="10.5" r="6.5" /><path d="m15.5 15.5 5 5" /></>,
	bell: <><path d="M18 9a6 6 0 1 0-12 0c0 5-2 6.5-2 6.5h16S18 14 18 9Z" /><path d="M10.3 19a2 2 0 0 0 3.4 0" /></>,
	star: <path d="m12 3.5 2.7 5.6 6.1.9-4.4 4.3 1 6.1-5.4-2.9-5.4 2.9 1-6.1-4.4-4.3 6.1-.9Z" />,
	chevronRight: <path d="m9 5 7 7-7 7" />,
	chevronDown: <path d="m5 9 7 7 7-7" />,
	chevronLeft: <path d="m15 5-7 7 7 7" />,
	dots: <><circle cx="5" cy="12" r="1.4" /><circle cx="12" cy="12" r="1.4" /><circle cx="19" cy="12" r="1.4" /></>,
	x: <path d="m5 5 14 14M19 5 5 19" />,
	check: <path d="m4.5 12.5 5 5 10-11" />,
	alertTriangle: <><path d="M12 4 2.8 20h18.4Z" /><path d="M12 10v4M12 17.2h.01" /></>,
	alertCircle: <><circle cx="12" cy="12" r="9" /><path d="M12 7.5v5M12 16.2h.01" /></>,
	info: <><circle cx="12" cy="12" r="9" /><path d="M12 11v5.5M12 7.8h.01" /></>,
	refresh: <><path d="M20 12a8 8 0 1 1-2.6-5.9" /><path d="M20 4v4.5h-4.5" /></>,
	trash: <><path d="M4 7h16M9.5 7V4.5h5V7" /><path d="M6.5 7 7.5 20h9L17.5 7" /><path d="M10.5 11v5M13.5 11v5" /></>,
	edit: <><path d="M12 5H6a2 2 0 0 0-2 2v11a2 2 0 0 0 2 2h11a2 2 0 0 0 2-2v-6" /><path d="M17.5 3.5a2.1 2.1 0 0 1 3 3L12.5 14.5l-4 1 1-4Z" /></>,
	plus: <path d="M12 5v14M5 12h14" />,
	externalLink: <><path d="M14 4h6v6" /><path d="m20 4-9 9" /><path d="M18 14v5a1.5 1.5 0 0 1-1.5 1.5H5A1.5 1.5 0 0 1 3.5 19V7.5A1.5 1.5 0 0 1 5 6h5" /></>,
	server: <><rect x="3" y="4" width="18" height="7" rx="1.6" /><rect x="3" y="13" width="18" height="7" rx="1.6" /><path d="M7 7.5h.01M7 16.5h.01" /></>,
	hardDrive: <><path d="M3.5 12h17" /><path d="M5.6 5h12.8l2.1 7v5.5a1.5 1.5 0 0 1-1.5 1.5H5a1.5 1.5 0 0 1-1.5-1.5V12Z" /><path d="M7 15.5h.01M10.5 15.5h.01" /></>,
	clock: <><circle cx="12" cy="12" r="9" /><path d="M12 7v5.2l3.2 2" /></>,
	filter: <path d="M3.5 5h17l-6.6 7.6V19l-3.8 2v-8.4Z" />,
	arrowLeft: <><path d="M20 12H4" /><path d="m10 6-6 6 6 6" /></>,
	menu: <path d="M4 7h16M4 12h16M4 17h16" />,
	sliders: <><path d="M4 8h10M18 8h2M4 16h4M12 16h8" /><circle cx="16" cy="8" r="2" /><circle cx="10" cy="16" r="2" /></>,
	folder: <path d="M3.5 6.5A1.5 1.5 0 0 1 5 5h4l2 2.5h8A1.5 1.5 0 0 1 20.5 9v9a1.5 1.5 0 0 1-1.5 1.5H5A1.5 1.5 0 0 1 3.5 18Z" />,
	terminal: <><rect x="3" y="4.5" width="18" height="15" rx="2" /><path d="m7.5 9.5 3 3-3 3M13 15.5h4" /></>,
	logOut: <><path d="M15 5.5V4a1.5 1.5 0 0 0-1.5-1.5h-8A1.5 1.5 0 0 0 4 4v16a1.5 1.5 0 0 0 1.5 1.5h8A1.5 1.5 0 0 0 15 20v-1.5" /><path d="M10 12h11" /><path d="m17.5 8.5 3.5 3.5-3.5 3.5" /></>,
	inbox: <><path d="M3.5 13.5h4l1.5 3h6l1.5-3h4" /><path d="M6 4.5h12l3 9v5a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 18.5v-5Z" /></>,
	pause: <><rect x="7" y="5" width="3.5" height="14" rx="1" /><rect x="13.5" y="5" width="3.5" height="14" rx="1" /></>,
	play: <path d="M7 4.5 19 12 7 19.5Z" />,
	compass: <><circle cx="12" cy="12" r="9" /><path d="m15.5 8.5-2 5-5 2 2-5Z" /></>,
	scale: <><path d="M12 4v16M7 20h10" /><path d="M4 9h16M6.5 9 4 14.5h5ZM17.5 9 15 14.5h5Z" /></>,
} as const
