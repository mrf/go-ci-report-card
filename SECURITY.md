# Security policy

## Supported versions

Until the project reaches version 1.0, security fixes are made on the latest
released version and the default branch only.

| Version | Supported |
|---|---|
| Latest release | Yes |
| Default branch | Yes |
| Older releases | No |

## Report a vulnerability privately

Please do not open a public issue or pull request for a suspected vulnerability.
Use the repository’s **Security → Report a vulnerability** form, which creates a
private security advisory visible only to the reporter and maintainers.

Include:

- the affected version or commit;
- reproduction steps or a minimal proof of concept;
- the expected and observed behavior;
- the likely impact; and
- any suggested mitigation.

Maintainers will acknowledge a complete report as soon as practical, keep the
reporter informed, and coordinate disclosure after a fix is available. Please
avoid accessing data that is not yours, disrupting services, or publicly
disclosing the issue before that coordination is complete.

## Security model

Go CI Report Card runs repository code inside GitHub-hosted Actions runners and
publishes static files. It has no application server, database, authentication,
analytics, or long-lived credential. Pull requests do not execute the Pages
upload and deployment steps.

Project maintainers remain responsible for reviewing custom check commands and
third-party Actions before enabling them.

