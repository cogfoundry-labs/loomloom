# Security policy

loomloom is in **beta** and under active development. Security fixes are applied to the latest release on the `main` branch. Please use the most recent version before reporting an issue.

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Report security issues privately using one of the following channels:

- **GitHub** — open a private [security advisory](https://github.com/cogfoundry-labs/loomloom/security/advisories/new)
- **Email** — send details to engineering@cogfoundry.ai

Please include:

- A description of the vulnerability and its potential impact
- Steps to reproduce or a proof of concept
- Affected version(s) and environment
- Any suggested fix, if available

Do not include real access tokens, API keys, or other secrets in your report.

## What to expect

- We aim to acknowledge security reports within a few business days.
- We will keep you informed during investigation and remediation.
- Please allow reasonable time for fixes before public disclosure.
- We will credit security researchers for valid reports when requested.

## Using loomloom securely

loomloom executes AI work through a remote execution platform. Follow these guidelines to keep your credentials, data, and operations secure.

### Protect your access token

- Only send `LOOMLOOM_TOKEN` to the configured `LOOMLOOM_SERVER` or the server explicitly provided with `--server`.
- Always use HTTPS.
- Never share access tokens publicly or store them in insecure locations.

### Verify the server endpoint

The default production endpoints are:

| Execution platform | Server endpoint |
|:---|:---|
| **CogFoundry** | `https://loomloom.cogfoundry.ai/loom/v1` |
| **ShengSuanYun** | `https://loomloom.shengsuanyun.com/loom/v1` |


Never send credentials to an unexpected host. Avoid following redirects that change the destination server.

### Handle data carefully

- Workbook contents are transmitted as Base64-encoded data inside JSON requests.
- Avoid logging or printing complete request payloads.
- Treat returned `accessUrl` values as temporary signed URLs.
- Do not store temporary URLs in long-term logs or share them publicly.

### Require user confirmation

AI agents should always obtain explicit user approval before:

- Performing paid operations
- Modifying remote state
- Publishing or unpublishing resources
- Submitting files or running remote workloads

Examples include:

- `submit-file`
- `run submit`
- `template-spec run`
- `market run`
- `listing publish`
- `listing unlist`

### Retry with care

Do not blindly retry paid or state-changing operations after an ambiguous failure.

Before retrying:

- Check the current run, listing, review, or usage state.
- Reuse `--client-request-id` only when retrying the exact same request.
- Use a new request ID when the payload or operation changes.
