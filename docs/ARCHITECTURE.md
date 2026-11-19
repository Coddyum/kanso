# Architecture

Kanso is one Go binary (a local web server and the `kanso` command), a web page embedded in it, a small
Mac app around that page, and a Swift helper that reads documents. Everything runs on the user's Mac.
Go standard library only; the page uses plain ES modules, no framework and no build step.

```text
            launchd (LaunchAgent, at login)
                 │
        bin/kanso-launcher ── restarts ──▶ bin/kanso serve ◀── HTTP 127.0.0.1 ── Kanso.app (WKWebView + notifications)
                                               │                                  └─ or any browser
             ┌─────────────────────────────────┼───────────────────────────────┐
         the user's folder               ~/Library/…                   internal/netgate
     (invoices + .kanso/db.json)   (settings, cache, log)        (ECB rates, brand logos only)
```

## Packages

| Package | Role |
| --- | --- |
| `cmd/kanso` | the binary: `serve` (default), `init`, `update`, `uninstall`, `doctor`, `demo`, `version` |
| `cmd/kanso-launcher` | started by launchd; starts `kanso serve`, starts it again when it stops, restarts it when `bin/kanso` is rebuilt |
| `internal/server` | HTTP only: routes, the request guard, the cached model; calls the packages below, writes JSON |
| `internal/ledger` | the business: payments, invoices grouped, due dates paired with invoices, suggestions, reserve forecast, analytics, reminders, edits (`Book`), the first-run checklist |
| `internal/folder` | the user's folder: its index (hash and text of each file, cached) and safe file operations (`Root`: no path out of the folder, no overwrite) |
| `internal/extract` | reads what a document says about itself: kind, vendor, date, amount, number, billed period |
| `internal/vendor` | the vendor catalog: built-in vendors plus the user's `vendors.json`, billing pages |
| `internal/bank` | bank statements (CSV, OFX): columns, dates and amounts in any format, balance, recurring debits |
| `internal/intake` | folders dropped into the app: copied aside, sorted, copied in, undone |
| `internal/demo` | the demo folder: made-up invoices (real, readable PDFs) and a database that follows them |
| `internal/deadlines` | French paperwork: URSSAF, CFE, income tax, public holidays |
| `internal/currency` · `internal/rates` | the ECB's currencies · its daily rates, refreshed in the background |
| `internal/logo` | a vendor's icon, taken from its own site, cached |
| `internal/netgate` | the only way out of the Mac (see below) |
| `internal/i18n` | the server's texts, from the page's language files |
| `internal/store` | one JSON file, written atomically, with daily backups |
| `internal/config` | `~/Library/Application Support/Kanso/config.json`: the folder, the port, demo mode |
| `internal/setup` | the LaunchAgent and the paths outside the folder |
| `internal/day` · `internal/norm` · `internal/brand` | days and month names · comparison keys · the app's name, in one place |
| `web/` | the page: `index.html`, `css/` (one sheet per area), `js/` (ES modules, `main.js` starts everything), `locales/` |
| `macos/app` | the Mac app: a window on the server, the reminders, the Downloads watch |
| `macos/pdftext.swift` | the reading helper: PDFKit for PDFs, Vision for scans and pictures |
| `tools/privacy` | the commit guard that keeps personal files and data out of the repository |

Every package has its tests; `make test` runs gofmt, `go vet` and `go test -race`.

## Flows

**Reading.** The folder is the source of truth. `folder.Index` walks it, hashes each file once and reads
the text of new PDFs and pictures with `bin/pdftext` (12 files per call, 4 calls at once); hashes and texts
are cached in `~/Library/Caches/kanso`. `ledger.Book.Model` reads the documents with the corrections of
`db.json`, groups them into invoices, adds the running subscriptions it finds, and pairs each due date
with its invoice. The server keeps that model until the folder's signature, the database or the day
changes: switching screens costs nothing.

**The page.** `main.js` loads the language, then `/api/state` and the current screen's data. Every
4 seconds it asks `/api/sig`: a new folder signature reloads the data, a new server start reloads the page.
Writes are `POST`/`DELETE` calls carrying the `X-Inc` header.

**Writing.** Edits go through `ledger.Book`, which validates them (`Invalid` errors carry the key of a
sentence in `web/locales`) and saves `db.json` atomically. Files dropped into the app go through
`folder.Root` (never outside the folder, never over another file); a dropped folder goes through
`intake.Desk` (copied aside in `.kanso/intake`, planned, then copied in with hard links, undoable).

**The bank statement** is the request's body of `POST /api/bank/preview`: read in memory by
`internal/bank`, never written anywhere. Only the subscriptions and the reserve the user confirms are
saved, by `POST /api/bank/apply`.

**The Mac app** loads the page in a `WKWebView`, schedules the reminders of `/api/reminders` as macOS
notifications (so they arrive with the app closed), and reports what only it knows (notifications allowed,
opening at login, Downloads watch) to `/api/app-status`. When the user allows it, it watches `~/Downloads`
and hands each new PDF or picture to `/api/intake/peek`; an invoice comes back as a notification with
"File it" and "Ignore".

## The request guard

The server listens on `127.0.0.1` only and refuses a request whose `Host` is not `localhost` or
`127.0.0.1` on its port (DNS rebinding), anything another website asks a browser to load
(`Sec-Fetch-Site`), and any write without the `X-Inc` header (a form posted by another site cannot set it).

## The network gate

`internal/netgate` is the only code that opens a connection to the outside. Each request names its
purpose, and the purpose allows its hosts:

| Purpose | Allowed |
| --- | --- |
| `Rates()` | `www.ecb.europa.eu`, HTTPS, default port |
| `Logo(domain)` | the brand's own domain and its subdomains, HTTPS; redirects must stay there |

A logo is fetched only for a domain the user's data names, and neither download happens when its switch
is off (Settings › Privacy). The gate's test parses every Go file, the page's scripts and the Swift code,
and fails when another network call appears.

## Languages

- `web/locales/<lang>.json`, in i18next's file format without the library: `fr.json` is the reference,
  `en`, `es`, `ja`, `zh` and `ar` have exactly its keys. Plurals follow `Intl.PluralRules`
  (`key_one`, `key_few`…).
- The page reads them with `web/js/i18n.js` (`t(key, vars)`, `data-i18n` attributes); Arabic turns the
  page right to left (`<html dir="rtl">`, the CSS uses logical properties).
- The server writes its own texts (deadlines, reminders, refusals, notifications, CLI) from the same
  files, embedded in the binary (`internal/i18n`), in the language the page sends (`X-Lang`), the one
  chosen in Settings, or the Mac's.
- The Mac app names its menus and notification buttons from the same files (`mac.*`); `scripts/app.sh`
  writes the sentence macOS shows when it asks for the Downloads folder in each language.
- `install.sh` reads its sentences with `plutil`.
- New month folders take the user's language (`folder.months`); the names of every language are
  recognised, so a change of language never makes a second folder for a month.
- The `internal/i18n` test fails on a key missing in a language, a key no code uses, a key the code
  uses that `fr.json` lacks, or a sentence written in the code instead of the language files.

## LaunchAgent and launcher

`kanso init` writes `~/Library/LaunchAgents/io.github.coddyum.kanso.server.plist`: at login, launchd
starts `bin/kanso-launcher`, which starts `bin/kanso serve` and starts it again when it stops. The launcher
watches `bin/kanso`: after `make build` (or `kanso update`) it restarts the server on the new binary within
2 to 3 seconds, and the open page reloads itself. The launcher stays the same binary from one update to the
next, so macOS asks only once for access to the Documents folder. The server reads its folder and port
from `config.json`; a change of folder or the demo mode saves the setting, then stops the server, which the
launcher starts again.

## Where things are

| What | Where |
| --- | --- |
| Invoices | the user's folder, `Year/Vendor/Month` for what the app files |
| Database, backups, the user's vendors, drops | the folder's hidden `.kanso/`: `db.json`, `backups/`, `vendors.json`, `intake/` |
| Settings | `~/Library/Application Support/Kanso/config.json` |
| Demo folder | `~/Library/Application Support/Kanso/demo/Demo` |
| Cache (texts, rates, logos) | `~/Library/Caches/kanso/` — safe to delete |
| Log | `~/Library/Logs/kanso.log` |
| Mac app | `~/Applications/Kanso.app` |
