# Running Pedal Balance Lab

The application has two processes:

- a Go API that decodes and analyses FIT files;
- a Svelte development server that serves the interface and proxies `/api` requests to Go.

## Requirements

- Go 1.26 or newer
- Node.js 22.12 or newer
- npm

Check the installed versions:

```sh
go version
node --version
npm --version
```

## First-time setup

From the project directory, install the frontend dependencies:

```sh
cd /Users/golan/dev/intervals/web-lr-analysis
npm install
```

Go dependencies are downloaded automatically when the backend is first run. To download them explicitly:

```sh
cd /Users/golan/dev/intervals/web-lr-analysis/backend
go mod download
```

## Start the application

Open two terminal windows.

### Terminal 1: Go backend

```sh
cd /Users/golan/dev/intervals/web-lr-analysis/backend
go run ./cmd/server
```

The API listens on `http://127.0.0.1:8080` by default. Its health endpoint is:

```sh
curl http://127.0.0.1:8080/api/health
```

### Terminal 2: Svelte frontend

```sh
cd /Users/golan/dev/intervals/web-lr-analysis
npm run dev
```

Open [http://127.0.0.1:5173](http://127.0.0.1:5173) in a browser. The development server forwards `/api` requests to the Go backend.

Stop either process with `Ctrl-C` in its terminal.

## Using the application

1. Drop a `.FIT` cycling activity onto the upload area, or click the area to select one.
2. Confirm the crank lengths and bucket settings.
3. Select **Analyse ride**.
4. Review the power buckets, power/cadence heatmap, and regression results.

Uploads are limited to 25 MB. A FIT file must contain power, cadence, and dual-sided left/right balance records. Uploaded files are decoded in memory and are not saved by the backend.

## Configuration

The backend accepts these environment variables:

| Variable | Default | Purpose |
| --- | --- | --- |
| `ADDR` | `127.0.0.1:8080` | Address and port used by the Go API |
| `CORS_ORIGIN` | unset | Permitted frontend origin when the frontend and API are hosted separately |

For example:

```sh
cd /Users/golan/dev/intervals/web-lr-analysis/backend
ADDR=127.0.0.1:9090 go run ./cmd/server
```

If the API port changes, update the `/api` proxy target in `vite.config.ts` before starting the frontend.

## Run the checks

Backend tests and static analysis:

```sh
cd /Users/golan/dev/intervals/web-lr-analysis/backend
go test ./...
go vet ./...
```

Frontend type checking and production build:

```sh
cd /Users/golan/dev/intervals/web-lr-analysis
npm run check
npm run build
```

The production frontend output is written to `dist/`.

## Troubleshooting

### The page opens but analysis fails

Make sure the Go backend is still running on `127.0.0.1:8080`, then check its health endpoint with the `curl` command above.

### Vite reports an unsupported Node.js version

Upgrade Node.js to 22.12 or newer, reinstall the frontend dependencies, and restart the development server.

### The FIT file has no usable records

The activity must contain simultaneous power, cadence, and FIT `left_right_balance` values. Single-sided meters and files that omit balance cannot produce this analysis.

### Port 5173 or 8080 is already in use

Stop the process using that port, or change the frontend port in `vite.config.ts` and/or the backend address with `ADDR`.
