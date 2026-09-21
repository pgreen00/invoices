import { getDb, transaction } from './db.js';

/* ------------------------------------------------------------------ */
/* Settings                                                            */
/* ------------------------------------------------------------------ */

const SETTINGS_FIELDS = [
  'business_name',
  'business_tagline',
  'address_line1',
  'address_line2',
  'city_state_zip',
  'email',
  'phone',
  'tax_id',
  'default_rate_cents',
  'default_terms_days',
  'payment_instructions',
  'notes',
  'footer_message',
];

export function getSettings() {
  return getDb().prepare('SELECT * FROM settings WHERE id = 1').get();
}

export function saveSettings(data) {
  const assignments = SETTINGS_FIELDS.map((field) => `${field} = ?`).join(', ');
  getDb()
    .prepare(`UPDATE settings SET ${assignments} WHERE id = 1`)
    .run(...SETTINGS_FIELDS.map((field) => data[field]));
}

/* ------------------------------------------------------------------ */
/* Clients                                                             */
/* ------------------------------------------------------------------ */

const CLIENT_FIELDS = [
  'name',
  'attn',
  'address_line1',
  'address_line2',
  'city_state_zip',
  'email',
  'default_project',
  'default_contract',
  'default_po',
  'default_rate_cents',
  'default_terms_days',
  'archived',
];

export function listClients({ includeArchived = false } = {}) {
  const sql = includeArchived
    ? `SELECT c.*, (SELECT COUNT(*) FROM invoices WHERE client_id = c.id) AS invoice_count
         FROM clients c ORDER BY c.archived, c.name COLLATE NOCASE`
    : `SELECT c.*, (SELECT COUNT(*) FROM invoices WHERE client_id = c.id) AS invoice_count
         FROM clients c WHERE c.archived = 0 ORDER BY c.name COLLATE NOCASE`;
  return getDb().prepare(sql).all();
}

export function getClient(id) {
  return getDb().prepare('SELECT * FROM clients WHERE id = ?').get(Number(id));
}

export function createClient(data) {
  const columns = CLIENT_FIELDS.join(', ');
  const placeholders = CLIENT_FIELDS.map(() => '?').join(', ');
  const info = getDb()
    .prepare(`INSERT INTO clients (${columns}, created_at) VALUES (${placeholders}, ?)`)
    .run(...CLIENT_FIELDS.map((field) => data[field]), new Date().toISOString());
  return Number(info.lastInsertRowid);
}

export function updateClient(id, data) {
  const assignments = CLIENT_FIELDS.map((field) => `${field} = ?`).join(', ');
  getDb()
    .prepare(`UPDATE clients SET ${assignments} WHERE id = ?`)
    .run(...CLIENT_FIELDS.map((field) => data[field]), Number(id));
}

export function deleteClient(id) {
  getDb().prepare('DELETE FROM clients WHERE id = ?').run(Number(id));
}

/* ------------------------------------------------------------------ */
/* Invoices                                                            */
/* ------------------------------------------------------------------ */

export const INVOICE_FIELDS = [
  'number',
  'client_id',
  'business_name',
  'business_tagline',
  'business_address_line1',
  'business_address_line2',
  'business_city_state_zip',
  'business_email',
  'business_phone',
  'business_tax_id',
  'client_name',
  'client_attn',
  'client_address_line1',
  'client_address_line2',
  'client_city_state_zip',
  'client_email',
  'project_name',
  'contract_ref',
  'po_number',
  'invoice_date',
  'period_start',
  'period_end',
  'terms_days',
  'due_date',
  'expenses_cents',
  'discount_cents',
  'payment_instructions',
  'notes',
  'footer_message',
];

const ITEM_FIELDS = ['position', 'work_date', 'description', 'detail', 'hours', 'rate_cents'];

const INVOICE_SUMMARY_SQL = `
  SELECT
    i.*,
    COALESCE((SELECT SUM(amount_cents) FROM line_items WHERE invoice_id = i.id), 0) AS subtotal_cents,
    COALESCE((SELECT SUM(hours)        FROM line_items WHERE invoice_id = i.id), 0) AS total_hours,
    COALESCE((SELECT SUM(amount_cents) FROM line_items WHERE invoice_id = i.id), 0)
      + i.expenses_cents - i.discount_cents AS total_cents
  FROM invoices i
`;

export function listInvoices() {
  return getDb()
    .prepare(`${INVOICE_SUMMARY_SQL} ORDER BY i.invoice_date DESC, i.id DESC`)
    .all();
}

export function getInvoice(id) {
  return getDb().prepare(`${INVOICE_SUMMARY_SQL} WHERE i.id = ?`).get(Number(id));
}

export function getLineItems(invoiceId) {
  return getDb()
    .prepare('SELECT * FROM line_items WHERE invoice_id = ? ORDER BY position, id')
    .all(Number(invoiceId));
}

export function invoiceStats() {
  return getDb()
    .prepare(
      `SELECT
         COUNT(*) AS count,
         COALESCE(SUM(total_cents), 0) AS total_cents,
         COALESCE(SUM(total_hours), 0) AS total_hours
       FROM (${INVOICE_SUMMARY_SQL})`
    )
    .get();
}

/** Next sequential number for a year, e.g. "2026-004". */
export function nextInvoiceNumber(year = new Date().getFullYear()) {
  const row = getDb()
    .prepare(
      `SELECT number FROM invoices
        WHERE number LIKE ?
        ORDER BY CAST(substr(number, 6) AS INTEGER) DESC
        LIMIT 1`
    )
    .get(`${year}-%`);

  const previous = row ? parseInt(row.number.slice(5), 10) : 0;
  const next = (Number.isFinite(previous) ? previous : 0) + 1;
  return `${year}-${String(next).padStart(3, '0')}`;
}

export function numberExists(number, exceptId = null) {
  const row = exceptId
    ? getDb().prepare('SELECT id FROM invoices WHERE number = ? AND id != ?').get(number, Number(exceptId))
    : getDb().prepare('SELECT id FROM invoices WHERE number = ?').get(number);
  return Boolean(row);
}

function insertItems(db, invoiceId, items) {
  const stmt = db.prepare(
    `INSERT INTO line_items (invoice_id, ${ITEM_FIELDS.join(', ')})
     VALUES (?, ${ITEM_FIELDS.map(() => '?').join(', ')})`
  );
  items.forEach((item, index) => {
    stmt.run(
      invoiceId,
      index,
      item.work_date ?? '',
      item.description ?? '',
      item.detail ?? '',
      Number(item.hours) || 0,
      Number(item.rate_cents) || 0
    );
  });
}

export function createInvoice(data, items) {
  return transaction((db) => {
    const now = new Date().toISOString();
    const columns = INVOICE_FIELDS.join(', ');
    const placeholders = INVOICE_FIELDS.map(() => '?').join(', ');

    const info = db
      .prepare(
        `INSERT INTO invoices (${columns}, generated_at, updated_at)
         VALUES (${placeholders}, ?, ?)`
      )
      .run(...INVOICE_FIELDS.map((field) => data[field]), now, now);

    const invoiceId = Number(info.lastInsertRowid);
    insertItems(db, invoiceId, items);
    return invoiceId;
  });
}

export function updateInvoice(id, data, items) {
  return transaction((db) => {
    const assignments = INVOICE_FIELDS.map((field) => `${field} = ?`).join(', ');
    db.prepare(`UPDATE invoices SET ${assignments}, updated_at = ? WHERE id = ?`).run(
      ...INVOICE_FIELDS.map((field) => data[field]),
      new Date().toISOString(),
      Number(id)
    );

    // Items are fully replaced: simpler and safer than diffing, and the
    // row counts here are tiny.
    db.prepare('DELETE FROM line_items WHERE invoice_id = ?').run(Number(id));
    insertItems(db, Number(id), items);
    return Number(id);
  });
}

export function deleteInvoice(id) {
  getDb().prepare('DELETE FROM invoices WHERE id = ?').run(Number(id));
}
