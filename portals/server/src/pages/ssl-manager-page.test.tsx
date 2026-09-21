// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { SSLManagerPage } from './ssl-manager-page'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

describe('SSLManagerPage', () => {
	test('request journey stays AutoSSL-only', async () => {
		api.mockImplementation((path: string) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'shop', primary_domain: 'shop.test' }] })
			}
			if (String(path).includes('/certificates')) return Promise.resolve({ items: [] })
			return Promise.resolve({ items: [] })
		})
		render(
			<MemoryRouter initialEntries={['/ssl/request?account=acc-1']}>
				<CapProvider caps={{ 'websites.read': true, 'websites.write': true }}>
					<Routes>
						<Route path="ssl" element={<SSLManagerPage />} />
						<Route path="ssl/request" element={<SSLManagerPage />} />
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)
		expect(await screen.findByRole('button', { name: 'Request AutoSSL' })).toBeInTheDocument()
		expect(screen.queryByText(/Custom PEM — labeled stub/)).not.toBeInTheDocument()
		expect(screen.queryByRole('button', { name: 'Install certificate' })).not.toBeInTheDocument()
	})

	test('install journey offers AutoSSL and custom PEM install', async () => {
		api.mockImplementation((path: string) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'shop', primary_domain: 'shop.test' }] })
			}
			if (String(path).includes('/certificates')) return Promise.resolve({ items: [] })
			return Promise.resolve({ items: [] })
		})
		render(
			<MemoryRouter initialEntries={['/ssl/install?account=acc-1']}>
				<CapProvider caps={{ 'websites.read': true, 'websites.write': true }}>
					<Routes>
						<Route path="ssl" element={<SSLManagerPage />} />
						<Route path="ssl/install" element={<SSLManagerPage />} />
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)
		expect(await screen.findByRole('heading', { name: /Install an SSL Certificate on a Domain/ })).toBeInTheDocument()
		expect(screen.getByRole('button', { name: 'Request AutoSSL' })).toBeInTheDocument()
		expect(screen.getByRole('button', { name: 'Install certificate' })).toBeInTheDocument()
		expect(screen.getByLabelText('Certificate PEM')).toBeInTheDocument()
		expect(screen.getByLabelText('Private key PEM')).toBeInTheDocument()
		expect(screen.getByLabelText('CA bundle PEM')).toBeInTheDocument()
		expect(screen.queryByText(/Custom PEM — labeled stub/)).not.toBeInTheDocument()
	})

	test('service SSL shows host inventory and install controls', async () => {
		api.mockImplementation((path: string) => {
			if (String(path) === '/api/v1/server/ssl/service') {
				return Promise.resolve({
					hostname: 'kelmor.host',
					items: [{
						id: 'director',
						label: 'Kelmor Director',
						hostname: 'kelmor.host',
						ports: [2087],
						status: 'missing',
						cert_path: '/var/lib/panel/certs/panel-portals.crt',
						note: 'No certificate is installed for this service on the host.',
					}],
				})
			}
			if (String(path) === '/api/v1/accounts') return Promise.resolve({ items: [] })
			return Promise.resolve({ items: [] })
		})
		render(
			<MemoryRouter initialEntries={['/ssl/service']}>
				<CapProvider caps={{ 'websites.read': true, 'server.settings.write': true }}>
					<Routes>
						<Route path="ssl/service" element={<SSLManagerPage />} />
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)
		expect(await screen.findByRole('heading', { name: 'Manage Service SSL Certificates' })).toBeInTheDocument()
		expect(screen.getByRole('table', { name: 'Service certificate inventory' })).toBeInTheDocument()
		expect(screen.getByText('Kelmor Director')).toBeInTheDocument()
		expect(screen.getByRole('button', { name: 'Install on host' })).toBeInTheDocument()
		expect(screen.queryByText(/Custom PEM — labeled stub/)).not.toBeInTheDocument()
		expect(screen.queryByText('Select an account')).not.toBeInTheDocument()
	})
})
