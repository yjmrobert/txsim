# txsim / MockPay CLI

**MockPay** is a universal, provider-agnostic payment simulation layer designed
for AI agents and developer testing environments. It's a local CLI-first
utility that lets you simulate complex payment workflows without live API
keys, real money, or internet access.

Agents and tests can invoke `mockpay` to get deterministic, provider-shaped
JSON responses (Stripe-compatible today; Generic mode for quick prototyping)
and can force specific outcomes — `card_declined`, `insufficient_funds`,
`rate_limited`, etc. — to exercise error-handling paths that are normally
hard to reach.

## Status

MVP: Generic and Stripe modes for `charge`, `refund`, `customer`, and
`subscribe`. The provider interface is designed so PayPal, internal
enterprise systems, and a crypto (wallet/gas) layer can drop in later.

## Install

```sh
go install github.com/yjmrobert/txsim/cmd/mockpay@latest
```

Or from source:

```sh
git clone https://github.com/yjmrobert/txsim
cd txsim
go build -o mockpay ./cmd/mockpay
./mockpay --help
```

## Global flags

| Flag            | Default                 | Description                                                         |
|-----------------|-------------------------|---------------------------------------------------------------------|
| `--mode`        | `generic`               | Provider shape: `generic` or `stripe`                               |
| `--status`      | `succeeded`             | Force a deterministic outcome (see below)                           |
| `--delay`       | `0`                     | Sleep N milliseconds before responding — simulates network latency  |
| `--state-file`  | `~/.mockpay/state.json` | Override the state file location                                    |
| `--pretty`      | `false`                 | Pretty-print JSON output (default is compact, token-efficient)      |

### Supported `--status` values

`succeeded` (default), `card_declined`, `insufficient_funds`, `expired_card`,
`processing_error`, `rate_limited`.

## Commands

### Customers

```sh
mockpay customer create --email alice@example.com --name Alice --mode stripe
mockpay customer get cus_<id>
mockpay customer list
```

### Charges

```sh
# Happy path, Stripe mode, 200 ms simulated latency
mockpay charge --amount 2500 --currency usd --customer cus_<id> \
  --mode stripe --delay 200

# Deterministic failure - exits with code 1 and emits a Stripe-shaped error
mockpay charge --amount 500 --currency usd --customer cus_<id> \
  --mode stripe --status card_declined
```

### Refunds

```sh
# Full refund
mockpay refund --charge ch_<id>

# Partial refund
mockpay refund --charge ch_<id> --amount 500
```

### Subscriptions

```sh
mockpay subscribe --plan gold --customer cus_<id> --mode stripe
```

### Reset

```sh
mockpay reset   # wipes all mock state
```

## Exit codes

| Code | Meaning                                                                  |
|------|--------------------------------------------------------------------------|
| `0`  | Success                                                                  |
| `1`  | Simulated payment failure (declined, insufficient_funds, etc.). JSON is still emitted on stdout. |
| `2`  | Usage error (bad flags, unknown customer, etc.)                          |
| `3`  | State/IO error                                                           |

Agents can branch on either the exit code or the JSON shape.

## State storage

MockPay persists mock state as JSON at `~/.mockpay/state.json`. The file is
protected by both an in-process mutex and a cross-process advisory file lock
(`gofrs/flock`) so it's safe to invoke MockPay from parallel CI jobs or
concurrent agent runs.

Reset anytime with `mockpay reset` — or just `rm ~/.mockpay/state.json`.

## Why MockPay?

- **Neutral.** Multiple provider aliases from one tool. Avoid per-provider
  mock code sprawl in test suites.
- **Agent-first.** Compact JSON by default to minimise tokens; clear,
  consistent error shapes for LLMs to parse.
- **Local-first.** Fully offline — no API keys, no rate limits, no data
  leaving the machine.
- **Deterministic.** `--status` flag lets tests cover hard-to-reach failure
  paths on demand.

## Development

```sh
go build ./...
go vet ./...
go test ./... -race
```

CI runs build + vet + test on every push and PR — see
`.github/workflows/ci.yml`.
