# Pedal Balance Lab

A Svelte + Go web application for analysing dual-sided cycling power from a dropped FIT activity file. It reports power buckets, a power/cadence heatmap with actual `L/R` values, and the regression model `L = a + bP + cC`, both as reported and corrected for unequal crank lengths.

See [INTERPRETING_RESULTS.md](INTERPRETING_RESULTS.md) for a detailed explanation of every result, the regression coefficients, and the limits of the analysis. See [RUNNING.md](RUNNING.md) for local setup and startup instructions.

## Run locally

Requirements: Go 1.26+ and Node 22.12+ (the current workspace's Node 22.0 still builds, but Vite prints an upgrade warning).

In one terminal:

```sh
cd backend
go run ./cmd/server
```

In another:

```sh
npm install
npm run dev
```

Open `http://127.0.0.1:5173`. Vite proxies `/api` to the Go server at `127.0.0.1:8080`.

## Checks

```sh
cd backend && go test ./...
npm run check
npm run build
```

FIT uploads are capped at 25 MB, decoded in memory, and never persisted. The backend accepts `ADDR` to change its listen address and `CORS_ORIGIN` when the frontend is hosted separately.

## FIT balance semantics

The decoder accepts record messages only when power, cadence, timestamp, and `left_right_balance` are valid. FIT encodes the contribution in the lower seven bits and uses the high bit to state that the value describes the right side; records lacking that flag are not guessed.
