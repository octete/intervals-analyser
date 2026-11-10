# lr-analysis (Go)

This is the Go port of the Rust `lr-analysis` CLI in the parent directory. It
uses the same Intervals.icu endpoints, calculations, defaults, and command-line
concepts, and has no third-party Go dependencies.

## Build and test

Go 1.22 or newer is required.

```sh
go test ./...
go vet ./...
go build -o lr-analysis ./cmd/lr-analysis
```

## Configuration

Use environment variables:

```sh
export INTERVALS_API_KEY='your-api-key'
export INTERVALS_ATHLETE_ID='your-athlete-id'
```

Or create `lr-analysis/config.toml` under the platform configuration directory,
or pass a file with `--config`:

```toml
api_key = "your-api-key"
athlete_id = "your-athlete-id"
```

Environment variables override file values. `INTERVALS_BASE_URL` is available
for local API testing.

## Commands

```sh
./lr-analysis activities --last 20

./lr-analysis analyse --last
./lr-analysis analyse --activity-id i12345678
./lr-analysis analyse --from 2026-01-01 --to 2026-08-15
./lr-analysis analyse --bucket-size 25 --min-samples 60 --min-duration 60

./lr-analysis heatmap --last
./lr-analysis heatmap --activity-id i12345678
./lr-analysis heatmap --power-bucket-size 25 --rpm-bucket-size 5
./lr-analysis heatmap --balance reported --no-color

./lr-analysis regression --last
./lr-analysis regression --activity-id i12345678
./lr-analysis regression --from 2026-01-01 --min-power 50
./lr-analysis regression --interaction
```

All analysis commands support `--activity-id`, `--last`, or `--from`/`--to`.
Crank defaults are 160 mm left, 165 mm right, and a 162.5 mm meter setting.

The heat map uses 50 W rows and 10 rpm columns by default. Cells show full L/R
values such as `58.0/42.0`, with ANSI colours when output is a terminal.

Regression fits the centred model:

```text
L - 50 = a + b × ((P - 200) / 100) + c × ((C - 85) / 10)
```

`--interaction` adds the scaled power/cadence product. Corrected balance is the
default for heat maps and regression; pass `--balance reported` for meter data.

## Intervals.icu details

- Personal keys use HTTP Basic auth with username `API_KEY`.
- Activities: `GET /api/v1/athlete/{id}/activities`.
- One activity: `GET /api/v1/activity/{id}`.
- Streams: `GET /api/v1/activity/{id}/streams.json` with
  `types=time,watts,cadence,left_right_balance`.
- `left_right_balance` is treated as right-side percentage and rendered as L/R.
