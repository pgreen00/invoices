import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import Handlebars from 'handlebars';

import { formatCents, formatHours, centsToInput } from './money.js';
import { formatLong, formatDayDate, formatRange, formatTimestamp } from './dates.js';

const VIEWS = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'views');
const CACHE_TEMPLATES = process.env.NODE_ENV === 'production';
const cache = new Map();

Handlebars.registerHelper('money', formatCents);
Handlebars.registerHelper('hours', formatHours);
Handlebars.registerHelper('rateInput', centsToInput);
Handlebars.registerHelper('dateLong', formatLong);
Handlebars.registerHelper('dateDay', formatDayDate);
Handlebars.registerHelper('dateRange', formatRange);
Handlebars.registerHelper('timestamp', formatTimestamp);

Handlebars.registerHelper('eq', (a, b) => a === b);
Handlebars.registerHelper('gt', (a, b) => Number(a) > Number(b));
Handlebars.registerHelper('or', (...args) => args.slice(0, -1).some(Boolean));
Handlebars.registerHelper('inc', (n) => Number(n) + 1);

/** Splits multi-line text into an array so templates can emit one node per line. */
Handlebars.registerHelper('lines', (text) =>
  String(text ?? '')
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
);

Handlebars.registerHelper('jsonScript', (value) =>
  new Handlebars.SafeString(
    JSON.stringify(value ?? null).replace(/</g, '\\u003c').replace(/>/g, '\\u003e')
  )
);

function template(name) {
  if (CACHE_TEMPLATES && cache.has(name)) return cache.get(name);
  const source = fs.readFileSync(path.join(VIEWS, `${name}.hbs`), 'utf8');
  const compiled = Handlebars.compile(source);
  if (CACHE_TEMPLATES) cache.set(name, compiled);
  return compiled;
}

/**
 * Renders `name` into `layout` and assigns it to the response.
 * Pass `{ layout: false }` for standalone pages such as the print view.
 */
const APP_NAME = 'Invoices';

export function render(ctx, name, data = {}, options = {}) {
  const layout = options.layout === undefined ? 'layout' : options.layout;
  const title = data.title ?? APP_NAME;
  const context = {
    ...data,
    path: ctx.path,
    // Avoids a redundant "Invoices — Invoices" on the index page.
    documentTitle: title === APP_NAME ? APP_NAME : `${title} — ${APP_NAME}`,
  };
  const body = template(name)(context);

  ctx.type = 'html';
  ctx.body = layout
    ? template(layout)({ ...context, body: new Handlebars.SafeString(body) })
    : body;
}
