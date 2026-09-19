import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../client'
import type { Job } from '../types'

export interface QueuedOperation {
	operation_id?: string
}

export function jobsHref (accountId?: string, jobId?: string) {
	const params = new URLSearchParams()
	if (accountId) params.set('account', accountId)
	if (jobId) params.set('selected', jobId)
	const search = params.toString()
	return search ? `/jobs?${search}` : '/jobs'
}

export function queuedOpMessage (result: QueuedOperation | void, fallback: string) {
	if (result?.operation_id) return `${fallback} Job ${result.operation_id}.`
	return fallback
}

interface QueuedOpNoticeProps {
	message: string
	accountId?: string
	jobId?: string
}

export function QueuedOpNotice ({ message, accountId, jobId }: QueuedOpNoticeProps) {
	const [job, setJob] = useState<Job | null>(null)
	useEffect(() => {
		if (!jobId) {
			setJob(null)
			return
		}
		let cancelled = false
		function load () {
			api<Job>(`/api/v1/jobs/${jobId}`).then((next) => {
				if (!cancelled && next && typeof next.state === 'string' && next.id) setJob(next)
			}).catch(() => {
				if (!cancelled) setJob(null)
			})
		}
		load()
		const timer = window.setInterval(load, 2500)
		return () => {
			cancelled = true
			window.clearInterval(timer)
		}
	}, [jobId])
	if (!message) return null
	const href = jobsHref(accountId, jobId)
	return (
		<p className="feedback" role="status">
			{message}
			{job ? <span className="job-watch"> Job is {job.state}{job.progress ? ` (${job.progress}%)` : ''}.</span> : null}
			{accountId || jobId ? <> <Link to={href}>Open Jobs</Link></> : null}
		</p>
	)
}
