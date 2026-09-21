# Invoices

A small self-hosted invoice generator for hourly, weekly billing. Koa +
Handlebars + `node:sqlite`, three dependencies, no build step. Invoices are
laid out for Letter paper and printed straight from Chrome.

## Requirements

Node 22.5+ (this uses the built-in `node:sqlite` module). Verified on Node 26.

## Setup

```sh
npm install
cp .env.example .env    # then set DATABASE_URL
npm start               # http://127.0.0.1:3000
```

`DATABASE_URL` is **required** and has no default. It points at a SQLite file
and accepts a plain path, a `~`-prefixed path, or a `file:` URL:

```
DATABASE_URL=/Users/you/Somewhere/invoices.db
DATABASE_URL=file:///Users/you/Somewhere/invoices.db
DATABASE_URL=~/Somewhere/invoices.db
```

The parent directory must already exist — the app will not create it, so a
typo fails loudly instead of quietly starting a second, empty database. The
file itself is created and migrated on first run, seeded with example
settings and one example client for you to replace.

`PORT` (3000) and `HOST` (127.0.0.1) are optional. There is no authentication;
keep it bound to loopback.

## Using it

**Settings** holds your business details, default hourly rate, default terms,
and the boilerplate payment/notes text. **Clients** holds each client's billing
address and optional per-client overrides for project, PO, rate, and terms.

**New invoice** prefills the current Mon–Fri week with five dated rows at the
applicable rate. Fill in hours and descriptions, and the totals update live.
Invoice numbers auto-increment per year (`2026-001`, `2026-002`, …) and the
field can be overridden. Due date is derived from invoice date + terms.

The list view has a **Repeat** action that opens a new invoice rolled forward
one week from an existing one, carrying the client, project, descriptions, and
rates but clearing the hours. That is the fastest path for a weekly cadence.

Clicking an invoice opens the print view. In Chrome's print dialog, turn off
**Headers and footers**; margins are already set by the stylesheet.

## Design notes

**Money is stored as integer cents**, never floats. Line item amounts are a
SQLite generated column (`ROUND(hours * rate_cents)`), so the stored amount can
never drift from the hours and rate it came from.

**Invoices snapshot their billing details.** Each invoice keeps its own copy of
your business block and the client's address as of the moment it was generated.
Changing Settings or editing a client does not rewrite past invoices. Deleting
a client leaves its invoices intact. The one exception: switching the client
dropdown while editing re-snapshots the new client's address, which is what you
would expect.

**`generated_at` is immutable**; edits only bump `updated_at`.

**Dates are plain `YYYY-MM-DD` strings** parsed as local calendar dates.
`new Date('2024-03-18')` would parse as UTC and shift a day in western
timezones, so it is avoided in both the server and browser code.

**`journal_mode = DELETE`** rather than WAL. The database stays a single
self-contained file at rest, with no `-wal`/`-shm` sidecars that a
folder-syncing service or a naive file copy could capture out of step with each
other. Assumes one writer at a time, which suits a single-user local app.

**The browser JavaScript is progressive enhancement only.** Live totals, the
due-date readout, and the add-row button are conveniences; the server
recomputes and revalidates everything on save.

## Tests

```sh
npm test
```

27 tests covering money and date helpers, page rendering, the full invoice
lifecycle (create, validate, edit, repeat, delete, cascade), auto-numbering,
and the snapshot guarantee. Uses the built-in Node test runner against a
throwaway database in a temp directory — no fixtures to clean up.

## Layout

```
src/
  server.js       boot: open db, listen
  app.js          Koa app assembly
  db.js           DATABASE_URL resolution, pragmas, migration, seed
  schema.sql      tables
  repo.js         all SQL
  invoiceForm.js  form parsing and validation
  money.js        integer-cent arithmetic
  dates.js        local-date helpers
  middleware.js   form body parser, static files, error page
  render.js       Handlebars setup and helpers
views/            .hbs templates (layout, invoices, clients, settings)
public/           app.css, print.css, app.js
```
