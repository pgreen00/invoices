/**
 * All monetary values are stored and computed as integer cents.
 * Floats only appear at the edges: parsing user input and formatting output.
 */

export function parseMoneyToCents(input) {
  if (input === null || input === undefined) return 0;
  const cleaned = String(input).trim().replace(/[$,\s]/g, '');
  if (!cleaned) return 0;
  const value = Number(cleaned);
  if (!Number.isFinite(value)) return 0;
  return Math.round(value * 100);
}

export function parseHours(input) {
  if (input === null || input === undefined) return 0;
  const cleaned = String(input).trim();
  if (!cleaned) return 0;
  const value = Number(cleaned);
  if (!Number.isFinite(value) || value < 0) return 0;
  // Keep two decimals so 15-minute increments stay exact.
  return Math.round(value * 100) / 100;
}

export function formatCents(cents) {
  const n = Number(cents) || 0;
  const negative = n < 0;
  const abs = Math.abs(n);
  const body = (abs / 100).toLocaleString('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
  return `${negative ? '-' : ''}$${body}`;
}

/** Cents -> a bare "165.00" suitable for a number input's value. */
export function centsToInput(cents) {
  return ((Number(cents) || 0) / 100).toFixed(2);
}

export function formatHours(hours) {
  return (Number(hours) || 0).toFixed(2);
}

export function computeTotals(items, expensesCents = 0, discountCents = 0) {
  let hours = 0;
  let subtotalCents = 0;

  for (const item of items) {
    const itemHours = Number(item.hours) || 0;
    const rate = Number(item.rate_cents) || 0;
    hours += itemHours;
    subtotalCents += Math.round(itemHours * rate);
  }

  hours = Math.round(hours * 100) / 100;
  const totalCents = subtotalCents + (expensesCents || 0) - (discountCents || 0);

  return {
    hours,
    subtotal_cents: subtotalCents,
    expenses_cents: expensesCents || 0,
    discount_cents: discountCents || 0,
    total_cents: totalCents,
  };
}
