# lr-analysis

`lr-analysis` is a small Rust CLI for analysing cycling power and left/right
balance from Intervals.icu activity streams. It groups samples by reported total
power (50 W buckets by default), reports time and sample statistics, and estimates
the effect of unequal crank lengths.

## Intervals.icu API details

The integration follows the current Intervals.icu OpenAPI documentation:

- Personal API keys use HTTP Basic authentication with username `API_KEY` and
  the API key as the password.
- Activities come from `GET /api/v1/athlete/{id}/activities`, using the required
  `oldest` query parameter and optional `newest` and `limit` parameters.
- Streams come from `GET /api/v1/activity/{id}/streams{ext}`. The CLI requests
  `.json` with `types=time,watts,left_right_balance`.
- When explicitly requested, Intervals.icu returns its fixed power stream as
  `watts`. The `left_right_balance` value is the percentage contributed by the
  right leg, so `48` is displayed as `52.0/48.0` (left/right).

Some source platforms strip cycling-dynamics data. Activities without a
`left_right_balance` stream are skipped with a diagnostic; the command fails if
none of the selected activities can be analysed.

## Configuration

Create an API key under Developer Settings in Intervals.icu, then use environment
variables:

```sh
export INTERVALS_API_KEY='your-api-key'
export INTERVALS_ATHLETE_ID='your-athlete-id'
```

Alternatively, create `lr-analysis/config.toml` inside the platform config
directory (for example `~/.config/lr-analysis/config.toml` on Linux), or pass a
file explicitly with `--config`:

```toml
api_key = "your-api-key"
athlete_id = "your-athlete-id"
```

Environment variables override config-file values. Credentials are never built
into the binary and should not be committed.

## Build and use

```sh
cargo build --release

lr-analysis activities --last 20
lr-analysis analyse --last
lr-analysis analyse --activity-id i12345678
lr-analysis analyse --from 2026-01-01
lr-analysis analyse --from 2026-01-01 --to 2026-08-15
lr-analysis analyse --bucket-size 25
lr-analysis analyse --min-samples 60 --min-duration 60
```

The crank defaults match this bike: 160 mm left, 165 mm right, with a 162.5 mm
single crank-length setting. They can be changed with `--left-crank`,
`--right-crank`, and `--configured-crank`.

## Crank-length correction

For each sample, reported total power is split using the reported balance. Each
side is then scaled by:

```text
estimated side power = reported side power * actual length / configured length
```

The corrected total and balance are recomputed from those two estimates. With an
even reported 50/50 split and 160/165 mm cranks, the estimate is about
49.23/50.77. This is a geometric estimate: it assumes each pedal derives torque
using the shared configured crank length. It does not correct calibration,
pedalling technique, drivetrain losses, or other meter-specific processing.

Buckets are always selected using the meter-reported total power so their ranges
remain directly comparable with Intervals.icu. Both reported and corrected mean
power are shown.

## Tests

```sh
cargo test
```

Tests cover inclusive/exclusive bucket boundaries, configurable bucket sizes,
minimum bucket thresholds, and crank-length correction.
