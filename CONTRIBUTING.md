# Contributing to Kanso

Thank you for helping. Kanso is small on purpose: one Go binary on the standard library, a web page with
no framework and no build step, a native Mac app. Contributions that keep it that way are the most welcome.

## Getting started

Requirements: macOS 13 or later, Apple's command line tools, Go (the version in `go.mod` or later).

```bash
git clone https://github.com/Coddyum/kanso.git
cd kanso
git config core.hooksPath .githooks   # the privacy guard runs before every commit
make test                             # gofmt, go vet, go test -race
```

To work without touching a real install, run the demo on its own port:

```bash
make build
./bin/kanso demo -port 48230          # made-up data, http://localhost:48230
```

`make build` rebuilds the server (a running install restarts on it within seconds); `make app` rebuilds
the Mac app, only needed when `macos/app` changes. The layout of the code is described in
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Rules the tests enforce

| Rule | Enforced by |
| --- | --- |
| **No personal data in the repository**: no PDF, CSV, OFX, photo or database; no IBAN, e-mail address, phone number, SIRET or home-folder path | `tools/privacy`, in the commit hook; `go run ./tools/privacy -all` reviews everything |
| **Every visible text goes through `web/locales`**: `fr.json` is the reference, every language has exactly its keys, no sentence is written in the code | `internal/i18n` tests |
| **The network goes through `internal/netgate` only**, to the ECB and to a brand's own site | `internal/netgate` tests, which read the Go, JavaScript and Swift sources |
| **Formatted, vetted, race-free** | `make test` |

When a check refuses something that is a false alarm (an example address in a test), write
`privacy:ok` on that line.

## Code

- **Go**: standard library only; no new dependency. The server is HTTP only (`internal/server`); the
  business lives in `internal/ledger`, the folder in `internal/folder`. Errors are wrapped with context;
  an error the user can fix carries the key of its sentence in `web/locales` (`ledger.Invalid`,
  `folder.Refused`).
- **Web**: plain ES modules, no framework, no bundler. A screen renders from `S` (`web/js/state.js`);
  actions are `data-action` attributes handled in `web/js/actions.js`. CSS uses logical properties so
  that Arabic works right to left.
- **Swift**: `macos/app/main.swift` for the window, the reminders and the Downloads watch;
  `macos/pdftext.swift` reads documents with PDFKit and Vision.
- Tests go next to the code they cover. A change of behaviour comes with its test.

## Translations

Every language file is `web/locales/<code>.json`. To review a translation, edit the values only: keys and
`{{variables}}` must stay as they are, and plural forms follow the language's rules (`_one`, `_other`;
Arabic also has `_zero`, `_two`, `_few`, `_many`). `make test` tells you when a key is missing or a
variable changed.

Adding a language: add its file with every key of `fr.json`, add its code to `i18n.Langs`
(`internal/i18n`), its month folder names (`folder.months`) to the recognised names of `internal/day`,
and its code to `CFBundleLocalizations` in `macos/app/Info.plist` (the Mac app reads its menus
and notification buttons from the language files at run time).

## Issues

Use one of the forms: **Bug report**, **Idea** or **Translation**. Never paste personal data in an issue
(an invoice, a bank line, a real amount, the path of your folder): reproduce in demo mode when you can.
A security problem is reported privately, see [SECURITY.md](SECURITY.md).

For a large change, open an issue first: it saves you work on something that would not be merged.

## Pull requests

1. Fork the repository, create a branch, open a pull request against `main`.
2. One subject per pull request, with what it changes and why. The template lists what to check.
3. `make test` green, and the privacy guard green.
4. Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/)
   (`feat:`, `fix:`, `docs:`, `refactor:`…).
5. For a change you can see, a screenshot taken in demo mode.

How a change lands on `main`: nobody pushes to it, the maintainer included. Every change goes through a
pull request whose CI check `test` is green, and is squash-merged by the maintainer. The CI of an
outside contributor runs once the maintainer has approved it.

By contributing, you agree that your contribution is released under the project's license, the
[GNU AGPL-3.0](LICENSE), and that you follow the [code of conduct](CODE_OF_CONDUCT.md).
