# Chansat

<p align="center">
  <img src="assets/mockup.png" alt="Chansat terminal interface mockup">
</p>

[![Tests](https://img.shields.io/github/actions/workflow/status/ahme-dev/chansat/test.yml?branch=main&label=tests)](https://github.com/ahme-dev/chansat/actions/workflows/test.yml)
[![Build](https://img.shields.io/github/actions/workflow/status/ahme-dev/chansat/build.yml?branch=main&label=build)](https://github.com/ahme-dev/chansat/actions/workflows/build.yml)
[![Code quality](https://img.shields.io/github/actions/workflow/status/ahme-dev/chansat/quality.yml?branch=main&label=code%20quality)](https://github.com/ahme-dev/chansat/actions/workflows/quality.yml)

Track your work hours across tasks, projects and rates from your terminal.

Chansat comes with:

- TUI controllable by vim motions, and mouse.
- CLI for automation and integration with other apps.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/ahme-dev/chansat/main/install.sh | sh
```

The installer verifies the release checksum and writes to `~/.local/bin`. Set
`CHANSAT_INSTALL_DIR` or `CHANSAT_VERSION` to override the destination or version.

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
