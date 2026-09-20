// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { CapProvider } from '../rbac'
import { HubPage } from './hub-page'
import {
	FEATURE_MANAGER_CTA,
	FEATURE_SHOWCASE_EMPTY_DETAIL,
	FEATURE_SHOWCASE_EMPTY_TITLE,
} from './feature-showcase-page'
import * as client from '../client'

afterEach(() => {
	cleanup()
	vi.restoreAllMocks()
})

function renderShowcase () {
	return render(
		<MemoryRouter initialEntries={['/section/system?tool=feature-showcase']}>
			<CapProvider caps={{ 'server.read': true, 'packages.read': true }}>
				<Routes>
					<Route path="section/:hubId" element={<HubPage />} />
				</Routes>
			</CapProvider>
		</MemoryRouter>,
	)
}

describe('Feature Showcase', () => {
	test('lists package capabilities and host apps with tool links', async () => {
		vi.spyOn(client, 'api').mockImplementation(async (path) => {
			if (path === '/api/v1/feature-sets') {
				return { items: [{ id: 'fs-1', name: 'full-hosting', features: { websites: true, email: false } }] }
			}
			if (path === '/api/v1/packages') {
				return { items: [{ id: 'pkg-1', name: 'Starter', feature_set_id: 'fs-1' }] }
			}
			if (path === '/api/v1/server/apps') {
				return { items: [{ id: 'phpmyadmin', label: 'phpMyAdmin', status: 'installed', description: 'SQL browser from the host package.' }] }
			}
			return { items: [] }
		})
		renderShowcase()
		expect(await screen.findByRole('heading', { name: 'Feature Showcase' })).toBeInTheDocument()
		expect(screen.getByRole('heading', { name: 'Package capabilities' })).toBeInTheDocument()
		expect(screen.getAllByText('MultiPHP Manager').length).toBeGreaterThan(0)
		expect(screen.getAllByText('Email Management').length).toBeGreaterThan(0)
		expect(screen.getByRole('link', { name: 'MultiPHP Manager' })).toHaveAttribute('href', '/websites')
		expect(screen.getAllByText('phpMyAdmin').length).toBeGreaterThan(0)
		expect(screen.getByRole('link', { name: 'phpMyAdmin' })).toHaveAttribute('href', '/section/sql?tool=phpmyadmin')
		expect(screen.queryByText(/This surface is part of the Director catalog/)).not.toBeInTheDocument()
		expect(screen.getAllByRole('link', { name: FEATURE_MANAGER_CTA })[0]).toHaveAttribute('href', '/features')
	})

	test('shows an honest empty catalog with a Feature Manager CTA', async () => {
		vi.spyOn(client, 'api').mockResolvedValue({ items: [] })
		renderShowcase()
		expect(await screen.findByText(FEATURE_SHOWCASE_EMPTY_TITLE)).toBeInTheDocument()
		expect(screen.getByText(FEATURE_SHOWCASE_EMPTY_DETAIL)).toBeInTheDocument()
		const featureManagerLinks = screen.getAllByRole('link', { name: FEATURE_MANAGER_CTA })
		expect(featureManagerLinks.length).toBeGreaterThan(0)
		expect(featureManagerLinks.every((link) => link.getAttribute('href') === '/features')).toBe(true)
		expect(screen.queryByRole('columnheader', { name: 'Capability' })).not.toBeInTheDocument()
		expect(screen.queryByText(/This surface is part of the Director catalog/)).not.toBeInTheDocument()
	})
})
