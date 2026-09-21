import { parseMoneyToCents, parseHours } from './money.js';
import { isISODate, addDays, todayISO } from './dates.js';

/** querystring.parse yields a string for one occurrence, an array for many. */
export function toArray(value) {
  if (value === undefined || value === null) return [];
  return Array.isArray(value) ? value : [value];
}

function str(value) {
  return String(value ?? '').trim();
}

function int(value, fallback = 0) {
  const parsed = parseInt(String(value ?? '').trim(), 10);
  return Number.isFinite(parsed) ? parsed : fallback;
}

export function dueDateFrom(invoiceDate, termsDays) {
  if (!isISODate(invoiceDate)) return '';
  return addDays(invoiceDate, Math.max(0, int(termsDays, 0)));
}

/**
 * Line items arrive as parallel arrays. Rows the user never touched
 * (no description, no detail, no hours) are dropped silently.
 */
export function parseItems(body) {
  const dates = toArray(body.item_date);
  const descriptions = toArray(body.item_description);
  const details = toArray(body.item_detail);
  const hours = toArray(body.item_hours);
  const rates = toArray(body.item_rate);

  const count = Math.max(
    dates.length,
    descriptions.length,
    details.length,
    hours.length,
    rates.length
  );

  const items = [];
  for (let i = 0; i < count; i += 1) {
    const description = str(descriptions[i]);
    const detail = str(details[i]);
    const parsedHours = parseHours(hours[i]);
    if (!description && !detail && parsedHours === 0) continue;

    const workDate = str(dates[i]);
    items.push({
      work_date: isISODate(workDate) ? workDate : '',
      description,
      detail,
      hours: parsedHours,
      rate_cents: parseMoneyToCents(rates[i]),
    });
  }
  return items;
}

export function businessSnapshot(settings) {
  return {
    business_name: settings.business_name ?? '',
    business_tagline: settings.business_tagline ?? '',
    business_address_line1: settings.address_line1 ?? '',
    business_address_line2: settings.address_line2 ?? '',
    business_city_state_zip: settings.city_state_zip ?? '',
    business_email: settings.email ?? '',
    business_phone: settings.phone ?? '',
    business_tax_id: settings.tax_id ?? '',
  };
}

export function clientSnapshot(client) {
  return {
    client_name: client?.name ?? '',
    client_attn: client?.attn ?? '',
    client_address_line1: client?.address_line1 ?? '',
    client_address_line2: client?.address_line2 ?? '',
    client_city_state_zip: client?.city_state_zip ?? '',
    client_email: client?.email ?? '',
  };
}

/**
 * Turns a submitted form into a fully typed row ready for SQLite, plus a
 * list of human-readable validation errors.
 *
 * `snapshots` supplies the business and client columns, which are snapshotted
 * rather than submitted with the form.
 */
export function buildInvoiceValues(body, snapshots) {
  const errors = [];

  const invoiceDate = str(body.invoice_date);
  if (!isISODate(invoiceDate)) errors.push('Invoice date is required.');

  const periodStart = str(body.period_start);
  const periodEnd = str(body.period_end);
  if (periodStart && periodEnd && periodEnd < periodStart) {
    errors.push('The service period ends before it starts.');
  }

  const clientId = int(body.client_id, 0);
  if (!clientId) errors.push('Select a client.');

  const termsDays = Math.max(0, int(body.terms_days, 0));
  const items = parseItems(body);
  if (items.length === 0) {
    errors.push('Add at least one line item with a description or hours.');
  }

  const values = {
    number: str(body.number),
    client_id: clientId || null,
    ...snapshots.business,
    ...snapshots.client,
    project_name: str(body.project_name),
    contract_ref: str(body.contract_ref),
    po_number: str(body.po_number),
    invoice_date: invoiceDate,
    period_start: isISODate(periodStart) ? periodStart : '',
    period_end: isISODate(periodEnd) ? periodEnd : '',
    terms_days: termsDays,
    due_date: dueDateFrom(invoiceDate, termsDays) || '',
    expenses_cents: parseMoneyToCents(body.expenses),
    discount_cents: parseMoneyToCents(body.discount),
    payment_instructions: String(body.payment_instructions ?? '').trim(),
    notes: String(body.notes ?? '').trim(),
    footer_message: String(body.footer_message ?? '').trim(),
  };

  return { values, items, errors };
}

export function parseClientBody(body) {
  const errors = [];
  const name = str(body.name);
  if (!name) errors.push('Client name is required.');

  const rate = str(body.default_rate);
  const terms = str(body.default_terms_days);

  return {
    errors,
    values: {
      name,
      attn: str(body.attn),
      address_line1: str(body.address_line1),
      address_line2: str(body.address_line2),
      city_state_zip: str(body.city_state_zip),
      email: str(body.email),
      default_project: str(body.default_project),
      default_contract: str(body.default_contract),
      default_po: str(body.default_po),
      default_rate_cents: rate ? parseMoneyToCents(rate) : null,
      default_terms_days: terms ? Math.max(0, int(terms, 0)) : null,
      archived: body.archived ? 1 : 0,
    },
  };
}

export function parseSettingsBody(body) {
  const errors = [];
  const name = str(body.business_name);
  if (!name) errors.push('Business name is required.');

  return {
    errors,
    values: {
      business_name: name,
      business_tagline: str(body.business_tagline),
      address_line1: str(body.address_line1),
      address_line2: str(body.address_line2),
      city_state_zip: str(body.city_state_zip),
      email: str(body.email),
      phone: str(body.phone),
      tax_id: str(body.tax_id),
      default_rate_cents: parseMoneyToCents(body.default_rate),
      default_terms_days: Math.max(0, int(body.default_terms_days, 15)),
      payment_instructions: String(body.payment_instructions ?? '').trim(),
      notes: String(body.notes ?? '').trim(),
      footer_message: String(body.footer_message ?? '').trim(),
    },
  };
}

export const helpers = { str, int, todayISO };
