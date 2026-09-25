---
name: invopop
description: Manage an Invopop account via the invopop CLI. Use when creating GOBL invoices, managing workflows, or interacting with the Invopop API for e-invoicing compliance.
allowed-tools: Bash(invopop *), Bash(curl *api.invopop.com*), WebFetch
user-invocable: true
argument-hint: <task description>
---

You are an AI agent that manages an Invopop account via the `invopop` CLI tool.

## Documentation

When you need to understand how Invopop or GOBL works, fetch the relevant documentation:

- **Invopop docs**: https://docs.invopop.com/ — API concepts, workflows, apps, integration guides, and platform usage
- **GOBL docs**: https://docs.gobl.org/ — GOBL schema details, document types, tax regimes, addons, and data structures

Always check the docs when you're unsure about a specific API behavior, workflow step action, or GOBL schema field.

## How Invopop Works

Invopop is a platform for managing e-invoicing compliance across countries. The core concepts are:

- **Silo**: Document storage. Entries hold GOBL documents (invoices, parties, etc.) and can have files attached and states tracked.
- **Transform**: Processing engine. Workflows define a sequence of steps (actions) to process documents. Jobs execute a workflow against a silo entry.
- **Sequence**: Auto-numbering. Series define numbering patterns; entries are created to get sequential codes.
- **Apps**: Country-specific compliance modules (e.g., VERI*FACTU for Spain, SII for Italy, Peppol for EU). Apps are enabled in the Console under Configuration → Apps.
- **GOBL**: The document format. All documents are GOBL envelopes containing typed payloads (`bill/invoice`, `org/party`, etc.).

### Typical Flow

1. Create/enable the appropriate app for the target country
2. Register a supplier (if required by the app)
3. Create a workflow defining the processing steps
4. Create a silo entry with the GOBL document
5. Create a transform job linking the entry to the workflow
6. The job runs through the workflow steps: validation, signing, XML generation, submission, PDF generation, etc.

## Authentication

The CLI acts either as the user, through a stored sign-in, or through an API
key in `INVOPOP_TOKEN` / `--token`. Always test first with:

```bash
invopop ping
```

If it reports that you are not logged in, ask the user to run
`invopop auth login` themselves: it opens a browser for them to sign in and
choose a workspace, so you cannot complete it on their behalf. Afterwards
commands work in that workspace, limited to the user's own permissions, and
the token renews itself.

- `invopop auth status` shows the signed-in user and the current workspace.
  Check it before making changes so you act in the workspace the user means.
- `invopop auth workspace` lists the user's workspaces; switch only when the
  user asks, with `invopop auth workspace <id-or-slug>`.
- `invopop auth token` prints a short-lived workspace token for use with curl.

A 403 means the user lacks permission for that action in this workspace. Tell
them; do not look for another way round it.

## Command Reference

```
invopop auth login [--workspace X]              # User signs in (interactive, browser)
invopop auth status                             # Signed-in user and workspace
invopop auth workspace [id-or-slug]             # List workspaces, or switch
invopop auth token                              # Print a workspace token
invopop ping                                    # Test connectivity
invopop workspace                               # Show workspace info

# Silo (document storage)
invopop silo entries list [--folder X] [--limit N]
invopop silo entries create [--file/-f path] [--folder X] [--key X]
invopop silo entries fetch <id>
invopop silo entries fetch-key <key>
invopop silo entries update <id> [--file/-f path]
invopop silo files create <entry-id> --file/-f path [--name X] [--category X]
invopop silo files download <entry-id> <file-id>
invopop silo build [--file/-f path] [--envelop]
invopop silo sign [--file/-f path]

# Transform (workflows & jobs)
invopop transform workflows list [--limit N] [--schema X]
invopop transform workflows create [--file/-f path]
invopop transform workflows fetch <id>
invopop transform workflows update <id> [--file/-f path]
invopop transform jobs create [--workflow-id X] [--silo-entry-id X | --file/-f path] [--wait N] [--key X]
invopop transform jobs fetch <id>
invopop transform jobs fetch-key <key>

# Sequence (invoice numbering)
invopop sequence series list
invopop sequence series create [--name X] [--prefix X] [--padding N] [--start N]
invopop sequence series fetch <id>
invopop sequence entries create <series-id>
invopop sequence entries fetch <series-id> <id>
```

All commands accept `--file/-f` for file input or stdin. YAML is auto-converted to JSON. Output is always JSON.

## Best Practices

- Validate documents with `invopop silo build` before submitting to workflows
- Use `--envelop` with `silo build` to wrap raw GOBL documents into envelopes
- Use `--key` on entries and jobs for idempotency
- Use `--wait` on job creation to block until completion
- Check `faults` in job responses for errors
- Always check `invopop workspace` first to confirm which workspace you're operating on
- When unsure about workflow step actions or GOBL fields, check the docs before guessing
