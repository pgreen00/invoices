import { after, before, describe, it } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const workdir = fs.mkdtempSync(path.join(os.tmpdir(), 'invoices-test-'));
process.env.DATABASE_URL = path.join(workdir, 'test.db');

const { openDatabase, resolveDatabasePath, ConfigError } = await import('../src/db.js');
const { createApp } = await import('../src/app.js');
const { computeTotals, parseMoneyToCents, formatCents } = await import('../src/money.js');
const { weekdaysFrom, formatRange, addDays } = await import('../src/dates.js');

let server;
let base;
let db;

before(async () => {
  db = openDatabase();
  server = createApp().listen(0, '127.0.0.1');
  await new Promise((resolve, reject) => {
    server.once('listening', resolve);
    server.once('error', reject);
  });
  base = `http://127.0.0.1:${server.address().port}`;
});

after(() => {
  server?.close();
  db?.close();
  fs.rmSync(workdir, { recursive: true, force: true });
});

const get = (p, init) => fetch(base + p, { redirect: 'manual', ...init });

function formPost(p, fields) {
  const params = new URLSearchParams();
  for (const [key, value] of fields) params.append(key, String(value));
  return fetch(base + p, {
    method: 'POST',
    redirect: 'manual',
    headers: { 'content-type': 'application/x-www-form-urlencoded' },
    body: params.toString(),
  });
}

describe('pure helpers', () => {
  it('parses money into integer cents', () => {
    assert.equal(parseMoneyToCents('165'), 16500);
    assert.equal(parseMoneyToCents('$1,234.56'), 123456);
    assert.equal(parseMoneyToCents('247.50'), 24750);
    assert.equal(parseMoneyToCents(''), 0);
    assert.equal(parseMoneyToCents('garbage'), 0);
  });

  it('formats cents without float drift', () => {
    assert.equal(formatCents(622875), '$6,228.75');
    assert.equal(formatCents(0), '$0.00');
    assert.equal(formatCents(-500), '-$5.00');
  });

  it('totals hours and amounts', () => {
    const totals = computeTotals(
      [
        { hours: 8, rate_cents: 16500 },
        { hours: 7.5, rate_cents: 16500 },
        { hours: 2, rate_cents: 24750 },
      ],
      2500,
      1000
    );
    assert.equal(totals.hours, 17.5);
    assert.equal(totals.subtotal_cents, 132000 + 123750 + 49500);
    assert.equal(totals.total_cents, totals.subtotal_cents + 2500 - 1000);
  });

  it('builds Mon–Fri from any day in the week', () => {
    // 2024-03-20 is a Wednesday.
    assert.deepEqual(weekdaysFrom('2024-03-20'), [
      '2024-03-18',
      '2024-03-19',
      '2024-03-20',
      '2024-03-21',
      '2024-03-22',
    ]);
    // Sunday belongs to the week that just ended.
    assert.equal(weekdaysFrom('2024-03-24')[0], '2024-03-18');
    assert.equal(addDays('2024-02-28', 2), '2024-03-01'); // leap year
  });

  it('formats date ranges compactly', () => {
    assert.equal(formatRange('2024-03-18', '2024-03-22'), 'Mar 18 – 22, 2024');
    assert.equal(formatRange('2024-03-28', '2024-04-01'), 'Mar 28 – Apr 1, 2024');
  });

  it('requires DATABASE_URL', () => {
    assert.throws(() => resolveDatabasePath(''), ConfigError);
    assert.throws(() => resolveDatabasePath('postgres://localhost/db'), ConfigError);
  });
});

describe('pages render', () => {
  for (const p of ['/', '/invoices/new', '/clients', '/clients/new', '/settings']) {
    it(`GET ${p} → 200`, async () => {
      const res = await get(p);
      assert.equal(res.status, 200, `${p} returned ${res.status}`);
      assert.match(res.headers.get('content-type'), /text\/html/);
    });
  }

  it('serves static assets', async () => {
    for (const asset of ['/assets/app.css', '/assets/print.css', '/assets/app.js']) {
      assert.equal((await get(asset)).status, 200, asset);
    }
  });

  it('blocks path traversal on /assets', async () => {
    const res = await get('/assets/../src/db.js');
    assert.notEqual(res.status, 200);
  });

  it('404s unknown routes', async () => {
    assert.equal((await get('/nope')).status, 404);
  });

  it('prefills the new-invoice form with five weekday rows', async () => {
    const html = await (await get('/invoices/new')).text();
    assert.equal(html.match(/name="item_date"/g).length, 5 + 1); // 5 rows + <template>
    assert.match(html, /Northwind Retail Group/);
  });
});

describe('invoice lifecycle', () => {
  let invoiceUrl;

  it('creates an invoice and auto-numbers it', async () => {
    const year = new Date().getFullYear();
    const res = await formPost('/invoices', [
      ['client_id', 1],
      ['number', ''],
      ['project_name', 'Order Management Platform'],
      ['invoice_date', '2024-03-24'],
      ['period_start', '2024-03-18'],
      ['period_end', '2024-03-22'],
      ['terms_days', '15'],
      ['expenses', '0'],
      ['discount', '0'],
      ['item_date', '2024-03-18'],
      ['item_description', 'Checkout service refactor'],
      ['item_detail', 'Extracted payment orchestration.'],
      ['item_hours', '8'],
      ['item_rate', '165.00'],
      ['item_date', '2024-03-23'],
      ['item_description', 'Emergency production hotfix'],
      ['item_detail', ''],
      ['item_hours', '2'],
      ['item_rate', '247.50'],
      // An untouched trailing row that should be discarded.
      ['item_date', '2024-03-25'],
      ['item_description', ''],
      ['item_detail', ''],
      ['item_hours', ''],
      ['item_rate', '165.00'],
      ['payment_instructions', 'ACH / Wire — Lone Star Bank'],
      ['notes', 'Net 15.'],
      ['footer_message', 'Thank you for your business.'],
    ]);

    assert.equal(res.status, 302);
    invoiceUrl = res.headers.get('location');
    assert.match(invoiceUrl, /^\/invoices\/\d+$/);

    const html = await (await get(invoiceUrl)).text();
    assert.match(html, new RegExp(`No\\. ${year}-001`));
    assert.match(html, /\$1,815\.00/); // 8×165 + 2×247.50
    assert.match(html, /10\.00/); // billable hours
    assert.match(html, /Apr 8, 2024/); // due date = invoice date + 15
    assert.match(html, /Mar 18 – 22, 2024/);
    assert.doesNotMatch(html, /Mar 25/); // blank row dropped
  });

  it('shows the invoice in the list with computed totals', async () => {
    const html = await (await get('/')).text();
    assert.match(html, /\$1,815\.00/);
    assert.match(html, /Northwind Retail Group/);
  });

  it('rejects an invoice with no client or line items', async () => {
    const res = await formPost('/invoices', [
      ['client_id', ''],
      ['invoice_date', '2024-03-24'],
      ['terms_days', '15'],
    ]);
    assert.equal(res.status, 422);
    const html = await res.text();
    assert.match(html, /Select a client/);
    assert.match(html, /at least one line item/);
  });

  it('rejects a duplicate invoice number', async () => {
    const year = new Date().getFullYear();
    const res = await formPost('/invoices', [
      ['client_id', 1],
      ['number', `${year}-001`],
      ['invoice_date', '2024-03-31'],
      ['terms_days', '15'],
      ['item_description', 'Work'],
      ['item_hours', '1'],
      ['item_rate', '100'],
    ]);
    assert.equal(res.status, 422);
    assert.match(await res.text(), /already in use/);
  });

  it('edits an invoice and recomputes the total', async () => {
    const id = invoiceUrl.split('/').pop();
    const res = await formPost(`/invoices/${id}`, [
      ['client_id', 1],
      ['number', ''],
      ['invoice_date', '2024-03-24'],
      ['period_start', '2024-03-18'],
      ['period_end', '2024-03-22'],
      ['terms_days', '30'],
      ['expenses', '125.50'],
      ['discount', '25'],
      ['item_date', '2024-03-18'],
      ['item_description', 'Checkout service refactor'],
      ['item_hours', '9'],
      ['item_rate', '165.00'],
    ]);
    assert.equal(res.status, 302);

    const html = await (await get(invoiceUrl)).text();
    assert.match(html, /\$1,585\.50/); // 9×165 + 125.50 − 25
    assert.match(html, /Apr 23, 2024/); // terms now Net 30
    assert.match(html, /Net 30/);
  });

  it('preserves generated_at while bumping updated_at', () => {
    const row = db.prepare('SELECT generated_at, updated_at FROM invoices LIMIT 1').get();
    assert.ok(row.updated_at >= row.generated_at);
  });

  it('rolls an invoice forward a week via Repeat', async () => {
    const id = invoiceUrl.split('/').pop();
    const html = await (await get(`/invoices/new?copy_from=${id}`)).text();
    assert.match(html, /value="2024-03-25"/); // period start + 7 days
    assert.match(html, /value="2024-03-29"/); // period end + 7 days
    assert.match(html, /Checkout service refactor/);
  });

  it('numbers the second invoice sequentially', async () => {
    const year = new Date().getFullYear();
    const res = await formPost('/invoices', [
      ['client_id', 1],
      ['number', ''],
      ['invoice_date', '2024-03-31'],
      ['terms_days', '15'],
      ['item_description', 'Follow-up work'],
      ['item_hours', '4'],
      ['item_rate', '165'],
    ]);
    assert.equal(res.status, 302);
    const html = await (await get(res.headers.get('location'))).text();
    assert.match(html, new RegExp(`No\\. ${year}-002`));
  });

  it('deletes an invoice and cascades its line items', async () => {
    const id = Number(invoiceUrl.split('/').pop());
    assert.equal((await formPost(`/invoices/${id}/delete`, [])).status, 302);
    assert.equal((await get(invoiceUrl)).status, 404);

    const orphans = db
      .prepare('SELECT COUNT(*) AS n FROM line_items WHERE invoice_id = ?')
      .get(id);
    assert.equal(orphans.n, 0);
  });
});

describe('clients and settings', () => {
  it('creates a client and offers it on the invoice form', async () => {
    const res = await formPost('/clients', [
      ['name', 'Contoso Logistics'],
      ['email', 'ap@contoso.test'],
      ['default_rate', '185.00'],
      ['default_terms_days', '30'],
    ]);
    assert.equal(res.status, 302);
    assert.match(await (await get('/invoices/new')).text(), /Contoso Logistics/);
  });

  it('rejects a client with no name', async () => {
    const res = await formPost('/clients', [['name', '  ']]);
    assert.equal(res.status, 422);
    assert.match(await res.text(), /Client name is required/);
  });

  it('saves settings without rewriting past invoices', async () => {
    const before = db.prepare('SELECT business_name FROM invoices LIMIT 1').get();

    const res = await formPost('/settings', [
      ['business_name', 'Renamed Software LLC'],
      ['default_rate', '200.00'],
      ['default_terms_days', '20'],
    ]);
    assert.equal(res.status, 302);

    const settings = db.prepare('SELECT * FROM settings WHERE id = 1').get();
    assert.equal(settings.business_name, 'Renamed Software LLC');
    assert.equal(settings.default_rate_cents, 20000);

    const afterRow = db.prepare('SELECT business_name FROM invoices LIMIT 1').get();
    assert.equal(afterRow.business_name, before.business_name);
  });
});
