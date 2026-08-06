# Contributing to loomloom

Thanks for your interest in contributing! loomloom is in **beta**, so things move quickly and feedback is especially valuable. These guidelines are intentionally light — when in doubt, open an issue and ask.

## Ways to contribute

- **Report a bug** — [open an issue](https://github.com/cogfoundry-labs/loomloom/issues) with steps to reproduce.
- **Request a feature** — describe the problem you're trying to solve, not just the solution.
- **Improve the docs** — fixes to the README, `docs/`, or examples are always welcome.
- **Submit code** — bug fixes and small improvements via pull request.

## Before you start

- **Search first.** Check [existing issues](https://github.com/cogfoundry-labs/loomloom/issues) and open pull requests to avoid duplicates.
- **Discuss big changes early.** For anything large or breaking, open an issue to align on the approach before writing code. This saves everyone time.
- **One change per pull request.** Keep PRs focused and reviewable.

## Making a change

1. **Fork** the repository and create a branch from `main` (e.g., `fix/typo-in-readme` or `feat/new-command`).
2. **Make your change.** Match the style and conventions of the surrounding code.
3. **Test locally.** Make sure the CLI still runs and any examples you touched work. Run `loomloom doctor` to confirm your environment is healthy.
4. **Commit** with a clear message describing what and why (e.g., `Fix broken link in getting-started guide`).
5. **Open a pull request** against `main`. Fill in what the change does and link any related issue (e.g., `Closes #123`).

## Pull request checklist

- [ ] The change is focused and does one thing.
- [ ] Docs and examples are updated if behavior changed.
- [ ] No secrets, API keys, or tokens are committed (see [Security](#security)).
- [ ] The PR description explains the motivation.

## Style

- Keep it simple and readable; prefer clarity over cleverness.
- Follow the existing formatting in the file you're editing.
- For Markdown, use `-` for bullet lists and wrap example commands in code blocks.

## Security

- Never commit real tokens, API keys, or credentials.
- Please review our [security policy](SECURITY.md) for vulnerability reporting and secure usage guidelines.

## Reporting issues

A good bug report includes:

- What you expected to happen and what actually happened.
- Exact steps or commands to reproduce it.
- Your OS, and the loomloom version (`loomloom --version`).
- Relevant error output (with any tokens or private data redacted).

## Community

- Questions and discussion: [Discord](https://discord.gg/cogfoundry)
- Troubleshooting: [docs/reference/troubleshooting.md](docs/reference/troubleshooting.md)

## License

By contributing, you agree that your contributions are licensed under the [Apache License 2.0](LICENSE), the same license that covers the open-source parts of this project.

---

Thanks for helping make loomloom better! 🧵
