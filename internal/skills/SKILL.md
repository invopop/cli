---
name: invocli
description: Manage an Invopop account via the invocli CLI. Use when creating GOBL invoices, registering suppliers, managing workflows, or interacting with the Invopop API for e-invoicing compliance (VeriFACTU, SII, Peppol, etc.).
allowed-tools: Bash(invocli *), Bash(curl *api.invopop.com*)
user-invocable: true
argument-hint: <task description>
---

You are an AI agent that manages an Invopop account via the `invocli` CLI tool.

## Authentication

The `INVOPOP_TOKEN` environment variable or `--token` flag must be set. Always test with:

```bash
invocli ping
```

If auth fails, ask the user to set their token.

## Command Reference

```
invocli ping                                    # Test connectivity
invocli workspace                               # Show workspace info

# Silo (document storage)
invocli silo entries list [--folder X] [--limit N]
invocli silo entries create [--file/-f path] [--folder X] [--key X]
invocli silo entries fetch <id>
invocli silo entries fetch-key <key>
invocli silo entries update <id> [--file/-f path]
invocli silo files create <entry-id> --file/-f path [--name X] [--category X]
invocli silo files download <entry-id> <file-id>
invocli silo build [--file/-f path] [--envelop]
invocli silo sign [--file/-f path]

# Transform (workflows & jobs)
invocli transform workflows list [--limit N] [--schema X]
invocli transform workflows create [--file/-f path]
invocli transform workflows fetch <id>
invocli transform workflows update <id> [--file/-f path]
invocli transform jobs create [--workflow-id X] [--silo-entry-id X | --file/-f path] [--wait N] [--key X]
invocli transform jobs fetch <id>
invocli transform jobs fetch-key <key>

# Sequence (invoice numbering)
invocli sequence series list
invocli sequence series create [--name X] [--prefix X] [--padding N] [--start N]
invocli sequence series fetch <id>
invocli sequence entries create <series-id>
invocli sequence entries fetch <series-id> <id>
```

All commands accept `--file/-f` for file input or stdin. YAML is auto-converted to JSON. Output is always JSON.

## Workflow: Issue a VeriFACTU Invoice (Spain)

### Prerequisites
1. VERI*FACTU Spain app enabled (Console → Configuration → Apps)
2. A registered supplier
3. An invoice workflow

### Invoice Workflow Template

```json
{
    "name": "VERI*FACTU invoice",
    "schema": "bill/invoice",
    "country": "ES",
    "steps": [
        {"id": "a0000001-0000-0000-0000-000000000001", "name": "Set State Processing", "action": "silo.state", "config": {"state": "processing"}},
        {"id": "a0000001-0000-0000-0000-000000000002", "name": "Add Sequential Code", "action": "sequence.enumerate", "config": {"name": "Verifactu", "padding": 6, "start": 1}},
        {"id": "a0000001-0000-0000-0000-000000000003", "name": "Sign Envelope", "action": "silo.close"},
        {"id": "a0000001-0000-0000-0000-000000000004", "name": "Generate XML", "action": "verifactu.generate"},
        {"id": "a0000001-0000-0000-0000-000000000005", "name": "Issue Invoice", "action": "verifactu.send"},
        {"id": "a0000001-0000-0000-0000-000000000006", "name": "Generate PDF", "action": "pdf", "config": {"locale": "es", "logo_height": 60}},
        {"id": "a0000001-0000-0000-0000-000000000007", "name": "Set State Sent", "action": "silo.state", "config": {"state": "sent"}}
    ],
    "rescue": [
        {"id": "a0000001-0000-0000-0000-00000000000e", "name": "Set State Error", "action": "silo.state", "config": {"state": "error"}}
    ]
}
```

### Supplier Registration Workflow Template

```json
{
    "name": "VERI*FACTU supplier registration",
    "schema": "org/party",
    "steps": [
        {"id": "b0000001-0000-0000-0000-000000000001", "name": "Register supplier", "action": "verifactu.register"},
        {"id": "b0000001-0000-0000-0000-000000000002", "name": "Set State", "action": "silo.state", "config": {"state": "processing"}},
        {"id": "b0000001-0000-0000-0000-000000000003", "name": "Wait for upload", "action": "verifactu.wait.upload"},
        {"id": "b0000001-0000-0000-0000-000000000004", "name": "Set State Registered", "action": "silo.state", "config": {"state": "registered"}},
        {"id": "b0000001-0000-0000-0000-000000000005", "name": "Wait for approval", "action": "verifactu.wait.approval", "config": {"response": "approve"}}
    ],
    "rescue": [
        {"id": "b0000001-0000-0000-0000-00000000000e", "name": "Unregister", "action": "verifactu.unregister"},
        {"id": "b0000001-0000-0000-0000-00000000000f", "name": "Set State Rejected", "action": "silo.state", "config": {"state": "rejected"}}
    ]
}
```

### Supplier Entry Template (org/party)

Upload to silo with `--folder suppliers`:

```json
{
  "$schema": "https://gobl.org/draft-0/org/party",
  "name": "Company Name S.L.",
  "tax_id": {"country": "ES", "code": "B12345678"},
  "people": [{
    "name": {"given": "Legal Rep Name", "surname": "Surname"},
    "identities": [{"key": "national", "code": "12345678A"}],
    "addresses": [{"num": "1", "street": "Street", "locality": "City", "region": "Region", "code": "28001", "country": "ES"}]
  }],
  "addresses": [{"num": "1", "street": "Street", "locality": "City", "region": "Region", "code": "28001", "country": "ES"}],
  "emails": [{"addr": "billing@example.com"}]
}
```

After triggering the registration workflow, documents must be uploaded via direct API calls:

```bash
# Download agreement
curl -H "Authorization: Bearer $INVOPOP_TOKEN" \
  "https://api.invopop.com/apps/verifactu/v1/entry/{entry_id}/agreement" -o agreement.pdf

# Upload signed agreement (base64 encode the PDF)
curl -X POST -H "Authorization: Bearer $INVOPOP_TOKEN" -H "Content-Type: application/json" \
  "https://api.invopop.com/apps/verifactu/v1/entry/{entry_id}/agreement" \
  -d '{"data": "<base64-pdf>", "signature": "hand"}'

# Upload ID front/back
curl -X POST -H "Authorization: Bearer $INVOPOP_TOKEN" -H "Content-Type: application/json" \
  "https://api.invopop.com/apps/verifactu/v1/entry/{entry_id}/identity" \
  -d '{"data": "<base64-image>", "view": "front"}'

curl -X POST -H "Authorization: Bearer $INVOPOP_TOKEN" -H "Content-Type: application/json" \
  "https://api.invopop.com/apps/verifactu/v1/entry/{entry_id}/identity" \
  -d '{"data": "<base64-image>", "view": "back"}'

# Confirm
curl -X POST -H "Authorization: Bearer $INVOPOP_TOKEN" \
  "https://api.invopop.com/apps/verifactu/v1/entry/{entry_id}/confirm"
```

### Invoice Template (bill/invoice)

Pipe directly into `transform jobs create --workflow-id <ID> --wait 60`:

```json
{
  "$schema": "https://gobl.org/draft-0/bill/invoice",
  "$regime": "ES",
  "$addons": ["es-verifactu-v1"],
  "type": "standard",
  "series": "SERIES-CODE",
  "issue_date": "2026-01-15",
  "currency": "EUR",
  "supplier": {
    "name": "Supplier S.L.",
    "tax_id": {"country": "ES", "code": "B12345678"},
    "addresses": [{"num": "1", "street": "Street", "locality": "City", "region": "Region", "code": "28001", "country": "ES"}]
  },
  "customer": {
    "name": "Customer S.L.",
    "tax_id": {"country": "ES", "code": "B87654321"}
  },
  "lines": [{
    "quantity": "1",
    "item": {"name": "Service description", "price": "1000.00"},
    "taxes": [{"cat": "VAT", "rate": "standard"}]
  }]
}
```

## Tax Scenarios (Spain)

| Scenario | Tax config |
|----------|-----------|
| Standard 21% | `{"cat": "VAT", "rate": "standard"}` |
| Reduced 10% | `{"cat": "VAT", "rate": "reduced"}` |
| Super-reduced 4% | `{"cat": "VAT", "rate": "super-reduced"}` |
| Reverse charge | `{"cat": "VAT", "key": "reverse-charge"}` |
| Intra-community | `{"cat": "VAT", "key": "intra-community"}` |
| Export | `{"cat": "VAT", "key": "export"}` |
| Exempt | `{"cat": "VAT", "key": "exempt"}` |

## Invoice Types

| Type | GOBL `type` | Notes |
|------|-------------|-------|
| Standard B2B (F1) | `standard` | Include customer with tax_id |
| Simplified B2C (F2) | `standard` + `$tags: ["simplified"]` | Under 400 EUR |
| Credit note (R1) | `credit-note` | Requires `preceding` array |
| Corrective (R1) | `corrective` | Requires `preceding` with tax totals |

## Best Practices

- Validate documents with `invocli silo build` before submitting to workflows
- Use `--envelop` with `silo build` to wrap raw GOBL documents into envelopes
- Use `--key` on entries and jobs for idempotency
- Use `--wait` on job creation to block until completion
- Check `faults` in job responses for errors
- The supplier `tax_id.code` in invoices must match a registered supplier
- Always check `invocli workspace` first to confirm which workspace you're operating on
