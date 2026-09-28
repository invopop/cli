# Invopop CLI

Command-line interface for the [Invopop](https://invopop.com) platform. Build and validate [GOBL](https://gobl.org) documents, manage silo entries and workflows, run transform jobs, and generate sequence numbers — all from your terminal.

## Installation

### Pre-built binaries

Download the latest release for your platform from the [releases page](https://github.com/invopop/cli/releases).

### From source

```bash
go install github.com/invopop/cli/cmd/invopop@latest
```

This produces a binary called `invopop` in your `$GOBIN`.

## Authentication

The CLI authenticates against the Invopop API using a token. Generate one from the [Invopop Console](https://console.invopop.com) under **Settings → Tokens**, then expose it to the CLI via either:

- the `INVOPOP_TOKEN` environment variable, or
- the `--token` / `-t` flag.

```bash
export INVOPOP_TOKEN=eyJhbGciOi...
invopop ping
```

The API host defaults to `https://api.invopop.com`. Set `--host` or the `INVOPOP_HOST`
environment variable for another environment, e.g. `https://api.invopop.dev` for local
development; a token only works on the environment that issued it.

## Commands

| Command | Description |
|---|---|
| `invopop ping` | Test API connectivity. |
| `invopop workspace` | Show details of the current workspace. |
| `invopop silo` | Manage silo entries, files, and GOBL documents. |
| `invopop transform` | Manage transform workflows and jobs. |
| `invopop sequence` | Manage sequence series and entries for invoice numbering. |
| `invopop skill` | Install the Invopop skill for [Claude Code](https://claude.com/claude-code). |

Run any command with `--help` for full flag and subcommand details.

## Examples

Build and validate a GOBL invoice locally:

```bash
invopop silo build --file invoice.yaml
```

Sign a GOBL document with the workspace's key:

```bash
invopop silo sign --file invoice.yaml
```

Create a silo entry from a local document:

```bash
invopop silo entries create --file invoice.yaml
```

Run a transform job against an existing entry and wait for completion:

```bash
invopop transform jobs create \
    --workflow-id 01h2x5... \
    --silo-entry-id 01h2x6... \
    --wait 30
```

Commands that take a document accept YAML or JSON on `--file`/`-f`, or on stdin. Results are printed to stdout as JSON, easy to pipe into `jq` or another command; `silo files download` writes the raw file instead.

## Claude Code integration

The CLI ships with a [Claude Code](https://claude.com/claude-code) skill that lets an AI agent drive your Invopop workspace through this binary. Install it with:

```bash
invopop skill install
```

Then invoke `/invopop` inside Claude Code.

## Documentation

- [Invopop docs](https://docs.invopop.com) — platform concepts, workflows, apps, and API reference.
- [GOBL docs](https://docs.gobl.org) — document schemas, tax regimes, and addons.
