#!/usr/bin/env node
// Drive Server Portal + Account Portal through the MVP surfaces in Chrome.
import { createRequire } from 'node:module'
import { setTimeout as delay } from 'node:timers/promises'

const require = createRequire(import.meta.url)
const puppeteer = require('puppeteer-core')

const SERVER = process.env.PANEL_SERVER_PORTAL || 'http://127.0.0.1:18443'
const ACCOUNT = process.env.PANEL_ACCOUNT_PORTAL || 'http://127.0.0.1:18444'
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

async function waitHTTP (host, want, tries = 40) {
	for (let i = 0; i < tries; i++) {
		try {
			const res = await fetch('http://127.0.0.1/', { headers: { Host: host } })
			if (res.status === want) return
		} catch {
			// retry
		}
		await delay(500)
	}
	throw new Error(`${host} never reached HTTP ${want}`)
}

const browser = await puppeteer.launch({
	executablePath: CHROME,
	headless: 'new',
	args: ['--no-sandbox', '--disable-dev-shm-usage', '--disable-gpu'],
})
const page = await browser.newPage()
page.setDefaultTimeout(20000)
try {
	await login(page, SERVER, ADMIN_USER, ADMIN_PASS)
	await page.waitForSelector('a[href="/accounts"]')
	await textIncludes(page, 'Host operations')
	await page.click('a[href="/accounts"]')
	await page.waitForSelector('input[name="username"]')
	await page.type('input[name="username"]', username)
	await page.type('input[name="domain"]', domain)
	await page.type('input[name="email"]', `ops@${domain}`)
	await page.type('input[name="password"]', OWNER_PASS)
	await page.click('button[type="submit"]')
	await page.waitForFunction((u) => document.body.innerText.includes(u) || document.body.innerText.includes('Queued'), {}, username)
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
	const accs = await (await fetch(`${API}/api/v1/accounts`, { headers: { Authorization: `Bearer ${token}` } })).json()
	const acc = (accs.items || []).find((a) => a.username === username)
	if (!acc) throw new Error('UI-created account missing from API')
	await page.goto(`${SERVER}/accounts/${acc.id}`, { waitUntil: 'networkidle0' })
	await textIncludes(page, username)
	await page.evaluate(() => {
		const b = [...document.querySelectorAll('button')].find((el) => el.textContent.trim() === 'Suspend')
		if (!b) throw new Error('suspend button missing')
		b.click()
	})
	await waitHTTP(domain, 503)
	await page.evaluate(() => {
		[...document.querySelectorAll('button')].find((el) => el.textContent.trim() === 'Unsuspend')?.click()
	})
	await waitHTTP(domain, 200)
	await page.goto(`${SERVER}/audit`, { waitUntil: 'networkidle0' })
	await textIncludes(page, 'account.suspend')
	console.log('UI_MVP_OK', username)
} finally {
	await browser.close()
}
