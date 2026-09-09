#!/usr/bin/env node
// Drive Kelmor Control against a running API: tenant login + one self-serve write.
import puppeteer from 'puppeteer-core'

const ACCOUNT = process.env.PANEL_ACCOUNT_PORTAL || 'https://127.0.0.1:39444'
const API = process.env.PANEL_API || 'http://127.0.0.1:29080'
const CHROME = process.env.CHROME || '/usr/local/bin/google-chrome'
const USER = process.env.PANEL_CONTROL_USER || 'freshhost'
const PASS = process.env.PANEL_CONTROL_PASSWORD || 'TenantPass!2026'
const DOMAIN = process.env.PANEL_SMOKE_DOMAIN || 'freshhost.test'
const LOCAL = process.env.PANEL_CONTROL_MAILBOX || 'control'
const MAIL_PASS = process.env.PANEL_CONTROL_MAIL_PASSWORD || 'ControlBox!2026'

async function login (page, url, user, pass) {
	await page.goto(url, { waitUntil: 'domcontentloaded' })
	const title = await page.title()
	if (title !== 'Kelmor Control') {
		throw new Error(`expected Kelmor Control document.title, got ${JSON.stringify(title)}`)
	}
	const body = await page.evaluate(() => document.body.innerText)
	if (!body.includes('Kelmor Control')) {
		throw new Error(`Control login missing Kelmor Control: ${body.slice(0, 400)}`)
	}
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

async function apiLogin () {
	let last = 'no attempt'
	for (let i = 0; i < 30; i++) {
		try {
			const res = await fetch(`${API}/api/v1/auth/login`, {
				method: 'POST',
				headers: { 'content-type': 'application/json' },
				body: JSON.stringify({ username: USER, password: PASS }),
			})
			const body = await res.json()
			if (body.token) return body.token
			last = JSON.stringify(body)
		} catch (err) {
			last = err instanceof Error ? err.message : String(err)
		}
		await new Promise((r) => setTimeout(r, 1000))
	}
	throw new Error(`tenant API login failed: ${last}`)
}

const browser = await puppeteer.launch({
	executablePath: CHROME,
	headless: true,
	ignoreHTTPSErrors: true,
	acceptInsecureCerts: true,
	args: ['--no-sandbox', '--disable-dev-shm-usage', '--disable-gpu', '--ignore-certificate-errors'],
})
const page = await browser.newPage()
page.setDefaultTimeout(30000)
try {
	await login(page, ACCOUNT, USER, PASS)
	await page.waitForFunction((d) => document.body.innerText.includes(d), {}, DOMAIN)
	const dash = await page.evaluate(() => document.body.innerText)
	if (!dash.includes('Kelmor Control') && !dash.includes(DOMAIN)) {
		throw new Error(`Control dashboard missing tenant state: ${dash.slice(0, 400)}`)
	}
	await page.click('a[href="/email"]')
	await page.waitForSelector('input[name="local_part"]')
	await page.type('input[name="local_part"]', LOCAL)
	await page.type('input[name="password"]', MAIL_PASS)
	await page.click('button[type="submit"]')
	await page.waitForFunction((local) => document.body.innerText.includes(local), {}, LOCAL)

	const token = await apiLogin()
	const me = await (await fetch(`${API}/api/v1/me`, { headers: { Authorization: `Bearer ${token}` } })).json()
	const accountId = (me.actor && me.actor.account_ids && me.actor.account_ids[0]) || ''
	if (!accountId) throw new Error('tenant /me has no account_id')
	let found = false
	for (let i = 0; i < 60; i++) {
		const boxes = await (await fetch(`${API}/api/v1/accounts/${accountId}/mail/mailboxes`, {
			headers: { Authorization: `Bearer ${token}` },
		})).json()
		found = (boxes.items || []).some((b) => b.local_part === LOCAL)
		if (found) break
		await new Promise((r) => setTimeout(r, 1000))
	}
	if (!found) throw new Error(`API missing mailbox ${LOCAL} after Control create`)

	await page.click('a[href="/ssl"]')
	await page.waitForFunction((d) => document.body.innerText.includes(d), {}, DOMAIN)
	const ssl = await page.evaluate(() => document.body.innerText)
	if (!ssl.includes(DOMAIN)) throw new Error(`Control SSL page missing ${DOMAIN}: ${ssl.slice(0, 400)}`)

	console.log('CONTROL_UI_MVP_OK', USER, `${LOCAL}@${DOMAIN}`)
} finally {
	await browser.close()
}
