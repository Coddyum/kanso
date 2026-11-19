# Security policy

Kanso keeps a micro-business's invoices and bank data on the user's Mac. A flaw that could expose them,
or let another website or program act on them, is taken seriously.

## Reporting a vulnerability

Please report it privately, through GitHub: **Security › Report a vulnerability** on this repository.
Do not open a public issue for a security problem.

Useful in a report: the version (`kanso version`), what an attacker can do, and the steps to reproduce it.
You will get an answer as soon as possible, and credit in the fix if you wish.

## Supported versions

Only the latest version of the `main` branch receives fixes: update with `kanso update`.

## In scope

- The local server (`127.0.0.1`): requests accepted from another website or another host, writes without
  the app's header, access to files outside the user's folder.
- The network gate: any request that reaches a host other than the European Central Bank or a brand's
  own site, or that carries the user's data.
- Data written where it should not be: a bank statement kept on disk, personal data in logs, caches or
  exports outside the user's folder and the app's own folders.
- The Mac app: content of a page or a document able to run code in the app.

## Out of scope

- An attacker who already controls the user's macOS account.
- The availability of the ECB's rates or of a brand's logo.
