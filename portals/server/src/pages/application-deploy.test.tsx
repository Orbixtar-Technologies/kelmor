// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { CapProvider } from '../rbac'
import { ServiceCreateForm } from './account-services-page'

afterEach(cleanup)

test('deploy form shows the four-step steps row', () => {
	render(<CapProvider caps={{ 'applications.write': true }}><ServiceCreateForm service="applications" canListWebsites resources={{ websites: [{ id: 'site-1', runtime: 'node', enabled: true, document_root: '/home/acme/app' }], applications: [] }} onCreate={vi.fn()} /></CapProvider>)
	expect(screen.getByText('Upload code')).toBeInTheDocument()
	expect(screen.getByText('Select website')).toBeInTheDocument()
	expect(screen.getByText('Configure')).toBeInTheDocument()
	expect(screen.getByText('Deploy')).toBeInTheDocument()
})

test('advanced options section is present in the deploy form', () => {
	render(<CapProvider caps={{ 'applications.write': true }}><ServiceCreateForm service="applications" canListWebsites resources={{ websites: [{ id: 'site-1', runtime: 'node', enabled: true, document_root: '/home/acme/app' }], applications: [] }} onCreate={vi.fn()} /></CapProvider>)
	expect(screen.getByText('Advanced options')).toBeInTheDocument()
})

test('deployment submits the website id and leaves detection to the agent', () => {
	const create = vi.fn().mockResolvedValue(undefined)
	render(<CapProvider caps={{ 'applications.write': true }}><ServiceCreateForm service="applications" canListWebsites resources={{ websites: [{ id: 'site-1', runtime: 'node', enabled: true, document_root: '/home/acme/app' }], applications: [] }} onCreate={create} /></CapProvider>)
	fireEvent.click(screen.getByRole('button', { name: 'Detect and deploy' }))
	expect(create).toHaveBeenCalledWith('applications', { website_id: 'site-1', runtime: 'node', start_command: '' })
})

test('shows empty state when no eligible Node or Python websites exist', () => {
	render(<CapProvider caps={{ 'applications.write': true }}><ServiceCreateForm service="applications" canListWebsites resources={{ websites: [], applications: [] }} onCreate={vi.fn()} /></CapProvider>)
	expect(screen.getByText(/No Node.js or Python websites found/)).toBeInTheDocument()
})

test('shows empty state when websites list has only php websites', () => {
	render(<CapProvider caps={{ 'applications.write': true }}><ServiceCreateForm service="applications" canListWebsites resources={{ websites: [{ id: 'site-php', runtime: 'php', enabled: true }], applications: [] }} onCreate={vi.fn()} /></CapProvider>)
	expect(screen.getByText(/No Node.js or Python websites found/)).toBeInTheDocument()
})

test('existing deployments cannot be used as website ids', () => {
	render(<CapProvider caps={{ 'applications.write': true }}><ServiceCreateForm service="applications" canListWebsites resources={{ websites: [{ id: 'site-1', runtime: 'node', enabled: true }], applications: [{ id: 'app-1', website_id: 'site-1' }] }} onCreate={vi.fn()} /></CapProvider>)
	expect(screen.queryByRole('button', { name: 'Detect and deploy' })).toBeNull()
	expect(screen.getByText(/Application already deployed/)).toBeInTheDocument()
})
