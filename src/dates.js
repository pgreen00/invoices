/**
 * Dates are stored as plain ISO `YYYY-MM-DD` strings and always parsed as
 * local calendar dates. `new Date('2024-03-18')` would parse as UTC and can
 * shift a day depending on the timezone, so it is avoided throughout.
 */

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const DAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

export function isISODate(value) {
  return typeof value === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(value);
}

export function toISO(date) {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, '0');
  const d = String(date.getDate()).padStart(2, '0');
  return `${y}-${m}-${d}`;
}

export function parseISO(iso) {
  if (!isISODate(iso)) return null;
  const [y, m, d] = iso.split('-').map(Number);
  const date = new Date(y, m - 1, d);
  return Number.isNaN(date.getTime()) ? null : date;
}

export function todayISO() {
  return toISO(new Date());
}

export function addDays(iso, days) {
  const date = parseISO(iso);
  if (!date) return iso;
  date.setDate(date.getDate() + days);
  return toISO(date);
}

/** Monday of the week containing `iso` (weeks run Monday–Sunday). */
export function mondayOfWeek(iso) {
  const date = parseISO(iso);
  if (!date) return iso;
  const dow = date.getDay(); // 0 = Sunday
  date.setDate(date.getDate() + (dow === 0 ? -6 : 1 - dow));
  return toISO(date);
}

/** The Mon–Fri window containing `iso`, defaulting to today. */
export function workWeekOf(iso = todayISO()) {
  const start = mondayOfWeek(iso);
  return { start, end: addDays(start, 4) };
}

/** Five ISO dates, Monday through Friday, starting from `startISO`'s week. */
export function weekdaysFrom(startISO) {
  const monday = mondayOfWeek(startISO);
  return [0, 1, 2, 3, 4].map((offset) => addDays(monday, offset));
}

/** "Mar 24, 2024" */
export function formatLong(iso) {
  const date = parseISO(iso);
  if (!date) return '';
  return `${MONTHS[date.getMonth()]} ${date.getDate()}, ${date.getFullYear()}`;
}

/** "Mon, Mar 18" */
export function formatDayDate(iso) {
  const date = parseISO(iso);
  if (!date) return '';
  return `${DAYS[date.getDay()]}, ${MONTHS[date.getMonth()]} ${date.getDate()}`;
}

/** "Mar 18 – 22, 2024" or "Mar 28 – Apr 1, 2024" */
export function formatRange(startISO, endISO) {
  const start = parseISO(startISO);
  const end = parseISO(endISO);
  if (!start && !end) return '';
  if (!start) return formatLong(endISO);
  if (!end) return formatLong(startISO);

  const sameYear = start.getFullYear() === end.getFullYear();
  const sameMonth = sameYear && start.getMonth() === end.getMonth();

  const left = `${MONTHS[start.getMonth()]} ${start.getDate()}`;
  const right = sameMonth
    ? `${end.getDate()}`
    : `${MONTHS[end.getMonth()]} ${end.getDate()}`;

  return sameYear
    ? `${left} – ${right}, ${end.getFullYear()}`
    : `${left}, ${start.getFullYear()} – ${right}, ${end.getFullYear()}`;
}

/** "Mar 24, 2024 at 2:14 PM" for the generated/updated timestamps. */
export function formatTimestamp(isoString) {
  if (!isoString) return '';
  const date = new Date(isoString);
  if (Number.isNaN(date.getTime())) return '';
  const time = date.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' });
  return `${MONTHS[date.getMonth()]} ${date.getDate()}, ${date.getFullYear()} at ${time}`;
}
