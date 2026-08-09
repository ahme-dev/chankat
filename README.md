# Chankat

<p align="center">
  <img src="assets/mockup.png" alt="Chankat terminal interface mockup">
</p>

[![Tests](https://img.shields.io/github/actions/workflow/status/ahme-dev/chankat/test.yml?branch=main&label=tests)](https://github.com/ahme-dev/chankat/actions/workflows/test.yml)
[![Build](https://img.shields.io/github/actions/workflow/status/ahme-dev/chankat/build.yml?branch=main&label=build)](https://github.com/ahme-dev/chankat/actions/workflows/build.yml)
[![Code quality](https://img.shields.io/github/actions/workflow/status/ahme-dev/chankat/quality.yml?branch=main&label=code%20quality)](https://github.com/ahme-dev/chankat/actions/workflows/quality.yml)

Track your work hours across tasks, projects and rates from your terminal.

Chankat comes with:

- TUI controllable by vim motions, and mouse.
- CLI for automation and integration with other apps.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/ahme-dev/chankat/main/install.sh | sh
```

The installer verifies the release checksum and writes to `~/.local/bin`. Set
`CHANKAT_INSTALL_DIR` or `CHANKAT_VERSION` to override the destination or version.
It also installs Bash completion under the user's data directory. Set
`CHANKAT_COMPLETION_DIR` to override that destination. Run
`chankat completion bash` to print the completion script directly. Completion
uses the CLI to suggest commands, flags and relevant record IDs.

On windows, please check releases and manually install.

## Releases

Merges to `main` are released from [Conventional Commits](https://www.conventionalcommits.org/):

- `fix:` creates a patch release.
- `feat:` creates a minor release.
- `BREAKING CHANGE:` creates a major release.
- Other commit types do not create a release.

Release archives are built for Linux, macOS and Windows.

## Screenshots

| Tasks | Payments |
| --- | --- |
| ![Tasks screen](assets/sc-tasks.png) | ![Payments screen](assets/sc-payments.png) |
| Project editor | Payment editor |
| ![Project editor](assets/sc-editor-projects.png) | ![Payment editor](assets/sc-editor-payments.png) |

## Contributing

Build and test changes with the Makefile:

```sh
make build
make test
```

Repository structure:

```text
cmd/                     application and development command entry points
internal/cli/            CLI parsing and output
internal/storage/        SQLite access, schema, validation and aggregation
internal/tui/            terminal application and tab navigation
internal/tui/components/ reusable TUI controls and formatting
internal/tui/screens/    TUI screens and forms
assets/                  screenshots and other static files
```

Keep command entry points limited to startup and dependency wiring. Put database
operations and data aggregation in `internal/storage`. CLI behavior belongs in
`internal/cli`; TUI screens belong in `internal/tui/screens`, with reusable UI
code in `internal/tui/components`. Keep tests beside the code they cover.

Set `CHANKAT_DATA_PATH` to override the SQLite database file. Development
commands use `.dev-data/data.sqlite`, leaving the installed application's user
database untouched:

```sh
make seed
make run
```
