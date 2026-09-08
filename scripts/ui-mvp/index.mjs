#!/usr/bin/env node
// Drive Server Portal + Account Portal through the MVP surfaces in Chrome.
import http from 'node:http'
import https from 'node:https'
import puppeteer from 'puppeteer-core'
import { setTimeout as delay } from 'node:timers/promises'

const SERVER = process.env.PANEL_SERVER_PORTAL || 'http://127.0.0.1:8443'
const ACCOUNT = process.env.PANEL_ACCOUNT_PORTAL || 'http://127.0.0.1:8444'
const API = process.env.PANEL_API || 'http://127.0.0.1:18080'
const CHROME = process.env.CHROME || '/usr/local/bin/google-chrome'
const ADMIN_USER = process.env.PANEL_ADMIN_USER || 'admin'
const ADMIN_PASS = process.env.PANEL_ADMIN_PASSWORD || 'ChangeMeOnce!2026'
const OWNER_PASS = 'TenantPass!2026'
const stamp = String(Math.floor(Date.now() / 1000))
const username = `ui${stamp}`
const domain = `${username}.test`

async function login (page, url, user, pass) {
	await page.goto(url, { waitUntil: 'domcontentloaded' })
	await page.waitForSelector('input[name="username"]')
	await page.click('input[name="username"]', { clickCount: 3 })
	await page.type('input[name="username"]', user)
	await page.click('input[name="password"]', { clickCount: 3 })
	await page.type('input[name="password"]', pass)
	await Promise.all([
		page.waitForNavigation({ waitUntil: 'networkidle0', timeout: 15000 }).catch(() => null),
		page.click('button[type="submit"]'),
	])
}

async function textIncludes (page, needle) {
	const body = await page.evaluate(() => document.body.innerText)
	if (!body.includes(needle)) {
		throw new Error(`page missing ${JSON.stringify(needle)}: ${body.slice(0, 400)}`)
	}
}

function httpHost (host) {
	return new Promise((resolve, reject) => {
		const req = http.request({
			host: '127.0.0.1',
			port: 80,
			path: '/',
			headers: { Host: host },
		}, (res) => {
			res.resume()
			if (res.statusCode === 301 || res.statusCode === 302) {
				const tls = https.request({
					host: '127.0.0.1',
					port: 443,
					path: '/',
					servername: host,
					headers: { Host: host },
					rejectUnauthorized: false,
				}, (sec) => {
					sec.resume()
					resolve(sec.statusCode)
				})
				tls.on('error', reject)
				tls.end()
				return
			}
			resolve(res.statusCode)
		})
		req.on('error', reject)
		req.end()
	})
}

async function waitHTTP (host, want, tries = 60) {
	for (let i = 0; i < tries; i++) {
		try {
			if (await httpHost(host) === want) return
		} catch {
			// retry
		}
		await delay(500)
	}
	throw new Error(`${host} never reached HTTP ${want}`)
}

const browser = await puppeteer.launch({
	executablePath: CHROME,
	headless: true,
	args: ['--no-sandbox', '--disable-dev-shm-usage', '--disable-gpu'],
})
const page = await browser.newPage()
page.setDefaultTimeout(20000)
try {
	await login(page, SERVER, ADMIN_USER, ADMIN_PASS)
	await page.waitForSelector('a[href="/accounts"]')
	await textIncludes(page, 'Host operations')
	await page.click('a[href="/accounts"]')
	await page.waitForSelector('select[name="package_id"] option')
	await page.waitForSelector('input[name="username"]')
	await page.type('input[name="username"]', username)
	await page.type('input[name="domain"]', domain)
	await page.type('input[name="email"]', `ops@${domain}`)
	await page.type('input[name="password"]', OWNER_PASS)
	await page.click('button[type="submit"]')
	await waitHTTP(domain, 200)
	await page.goto(`${SERVER}/accounts`, { waitUntil: 'networkidle0' })
	await textIncludes(page, username)

	await login(page, ACCOUNT, username, OWNER_PASS)
	await page.waitForFunction((d) => document.body.innerText.includes(d), {}, domain)
	await page.click('a[href="/files"]')
	await page.waitForSelector('textarea[name="content"]')
	const pathInput = await page.$('input[name="path"]')
	if (pathInput) {
		await pathInput.click({ clickCount: 3 })
		await pathInput.type('/public_html/ui-mvp.txt')
	}
	await page.type('textarea[name="content"]', 'written-from-account-portal')
	await page.evaluate(() => {
		const forms = [...document.querySelectorAll('form')]
		const write = forms.find((f) => f.querySelector('textarea[name="content"]'))
		write?.querySelector('button[type="submit"]')?.click()
	})
	await delay(800)
	await page.click('a[href="/email"]')
	await page.waitForSelector('input[name="local_part"]')
	await page.type('input[name="local_part"]', 'ui')
	await page.type('input[name="password"]', 'MailboxPass!2026')
	await page.click('button[type="submit"]')
	await page.click('a[href="/backups"]')
	await page.waitForFunction(() => document.body.innerText.includes('Create full backup'))
	await page.evaluate(() => {
		[...document.querySelectorAll('button')].find((b) => b.textContent.includes('Create full backup'))?.click()
	})
	await page.click('a[href="/ssl"]')
	await page.waitForSelector('input[name="hostname"]')
	await textIncludes(page, 'Request certificate')

	const tokenRes = await fetch(`${API}/api/v1/auth/login`, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ username: ADMIN_USER, password: ADMIN_PASS }),
	})
	const { token } = await tokenRes.json()
	async function accountByName (name) {
		const accs = await (await fetch(`${API}/api/v1/accounts`, { headers: { Authorization: `Bearer ${token}` } })).json()
		return (accs.items || []).find((a) => a.username === name)
	}
	async function waitAccountStatus (id, want, tries = 60) {
		for (let i = 0; i < tries; i++) {
			const row = await (await fetch(`${API}/api/v1/accounts/${id}`, { headers: { Authorization: `Bearer ${token}` } })).json()
			if (row.status === want) return row
			await delay(500)
		}
		throw new Error(`account ${id} never reached status ${want}`)
	}
	const acc = await accountByName(username)
	if (!acc) throw new Error('UI-created account missing from API')
	await page.goto(`${SERVER}/accounts/${acc.id}`, { waitUntil: 'networkidle0' })
	await textIncludes(page, username)
	await page.evaluate(() => {
		const b = [...document.querySelectorAll('button')].find((el) => el.textContent.trim() === 'Suspend')
		if (!b) throw new Error('suspend button missing')
		b.click()
	})
	await waitAccountStatus(acc.id, 'suspended')
	await waitHTTP(domain, 503)
	await page.evaluate(() => {
		const b = [...document.querySelectorAll('button')].find((el) => el.textContent.trim() === 'Unsuspend')
		if (!b) throw new Error('unsuspend button missing')
		b.click()
	})
	await waitAccountStatus(acc.id, 'active')
	await waitHTTP(domain, 200)
	const migUser = `mg${stamp}`
	const migDom = `${migUser}.test`
	await page.waitForSelector('input[name="migrate_username"]')
	await page.type('input[name="migrate_username"]', migUser)
	await page.type('input[name="migrate_domain"]', migDom)
	await page.evaluate(() => {
		const form = [...document.querySelectorAll('form')].find((f) => f.querySelector('input[name="migrate_username"]'))
		form?.querySelector('button[type="submit"]')?.click()
	})
	await waitHTTP(migDom, 200)
	await page.goto(`${SERVER}/audit`, { waitUntil: 'networkidle0' })
	await textIncludes(page, 'account.suspend')
	await textIncludes(page, 'account.migrate')
	console.log('UI_MVP_OK', username, migUser)
} finally {
	await browser.close()
}
