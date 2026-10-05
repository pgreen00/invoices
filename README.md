# Invoices

A small desktop invoice generator for hourly, weekly billing. Go + Gin +
`html/template` + SQLite, packaged as a native app with
[Wails](https://wails.io). It is completely offline: there is no server
process, nothing listens on a port, and the data never leaves your machine.
Invoices are laid out for Letter paper and printed (or saved as PDF) straight
from the app.

## Requirements

- Go 1.25+
- The Wails CLI, to build the app bundle:
  `go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0`
- macOS: Xcode command line tools. (Wails also targets Windows and Linux; see
  its [installation guide](https://wails.io/docs/gettingstarted/installation)
  for their dependencies.)

## Build and run

```sh
wails build -skipbindings     # → build/bin/Invoices.app
open build/bin/Invoices.app
```

`wails dev -skipbindings` runs it in development mode, rebuilding when Go files
change. There are no frontend bindings, hence `-skipbindings`.

## Where your data lives

The database is a single SQLite file in the app's data directory, created and
migrated on first launch and seeded with example settings and one example
client for you to replace:

| OS      | Path                                                  |
| ------- | ----------------------------------------------------- |
| macOS   | `~/Library/Application Support/Invoices/invoices.db`  |
| Windows | `%AppData%\Invoices\invoices.db`                      |
| Linux   | `$XDG_DATA_HOME/Invoices/invoices.db` (default `~/.local/share`) |

On macOS, **File → Show Database in Finder** reveals it. To back it up, copy
the file while the app is closed. To bring over a database from the old Node
version, quit the app and copy your file over `invoices.db`; the schema is
unchanged.

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

Clicking an invoice opens the print view. **Print / Save as PDF** (or ⌘P)
opens the system print panel on US Letter; margins come from the stylesheet,
and no browser headers or footers are added.

## How it works

The UI is a classic server-rendered multi-page app. Wails opens a native
window whose webview loads `wails://wails/` (macOS/Linux) or
`http://wails.localhost/` (Windows). Those requests never touch the network:
Wails hands each one to the Gin engine in-process as an ordinary
`http.Request`, and Gin's response goes straight back to the webview.

Two browser behaviours need help inside a webview, and `internal/desktop`
provides them:

- **Redirects.** WebKit does not follow 3xx responses from a custom URL
  scheme, and every form here posts then redirects. `FollowRedirects` turns a
  redirect into a tiny page that calls `location.replace`, so the POST still
  stays out of history.
- **`confirm()` and `print()` on macOS.** WKWebView ignores both unless the app
  implements them. `InstallWebViewHooks` adds a native confirm sheet (for the
  Delete buttons) and the system print panel to Wails' webview delegate.

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

**Dates are plain `YYYY-MM-DD` strings** treated as calendar dates, never
instants, so no timezone can shift them by a day — on the server or in the
browser JavaScript.

**`journal_mode = DELETE`** rather than WAL. The database stays a single
self-contained file at rest, with no `-wal`/`-shm` sidecars that a
folder-syncing service or a naive file copy could capture out of step with each
other. The app holds a single-instance lock and a single connection, so there
is only ever one writer.

**Form parsing matches the original JavaScript.** `internal/jscompat`
reproduces `Number()`, `parseInt()`, `Math.round()` and `trim()` semantics, so
input is parsed and rounded exactly as it was in the Node version.

**The browser JavaScript is progressive enhancement only.** Live totals, the
due-date readout, and the add-row button are conveniences; the server
recomputes and revalidates everything on save.

## Icons

The favicon is a clipboard drawn as a handful of rounded rectangles in
`public/icon.svg`. `build/appicon.svg` places the same artwork on the macOS
icon grid; `build/appicon.png` was rasterized from it with
`sips -s format png appicon.svg --out appicon.png`, and Wails generates the
platform icons from that PNG at build time.

## Tests

```sh
go test ./...
```

Covers money and date helpers, page rendering, the full invoice lifecycle
(create, validate, edit, repeat, delete, cascade), auto-numbering, the snapshot
guarantee, client archiving, the stylesheet invariants, and the webview
redirect adapter. Runs against a throwaway database in a temp directory.

## Layout

```
main.go               Wails app: data directory, window, menu
wails.json            Wails project config
build/                app icon and macOS Info.plist
internal/
  store/              SQLite: open, pragmas, schema.sql, seed, all SQL
  forms/              form parsing and validation
  money/              integer-cent arithmetic
  dates/              calendar-date helpers
  jscompat/           JavaScript number/string semantics
  web/                Gin engine, routes, rendering, error page
  desktop/            data directory, redirect adapter, macOS webview hooks
views/                html/template pages (layout, invoices, clients, settings)
public/               app.css, print.css, app.js, icons
```
