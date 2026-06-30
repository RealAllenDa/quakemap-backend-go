# quakemap-backend-go

Go refactor of the quakemap FastAPI backend. JMA earthquake, EEW, and tsunami
information comes exclusively from DMData's WebSocket.

## Run

```powershell
$env:REFRESH_TOKEN = "your-dmdata-refresh-token"
go run ./cmd/quakemap
```

The default address is `:9090`. Important environment variables:

| Variable | Default | Purpose |
|---|---|---|
| `ADDRESS` | `:9090` | HTTP listen address |
| `REFRESH_TOKEN` / `DMDATA_REFRESH_TOKEN` | empty | DMData OAuth refresh token |
| `DMDATA_CLIENT_ID` | JQuake client id | DMData OAuth client id |
| `DMDATA_CLIENT_SECRET` | JQuake client secret | DMData OAuth client secret |
| `KMONI_SHAKE_LEVEL_URL` | `https://kwatch-24h.net/EQLevel.json` | shake-level JSON source |
| `DEBUG` | `false` | expose fixture-driven `/debug` endpoints |
| `STATIC_DIR` | `static` | static asset directory |
| `PERSIST_DIR` | `persist` | restored/saved API state directory |
| `LOG_LEVEL` | `info` | Logrus level (`trace` through `panic`) |
| `LOG_COLORS` | `true` | enable colored, padded console logs |

Set `NO_COLOR` or `LOG_COLORS=false` when logs are redirected to a sink that
does not accept ANSI color sequences.

At startup the service creates `PERSIST_DIR` and restores `api_state.json` when
present. On graceful shutdown it saves the latest `/api/earthquake_info` and
`/api/tsunami_info` responses back to that file. Mount this directory as a
persistent volume when running in a container.

If no refresh token is set, HTTP and KMoni still run while the DMData heartbeat
reports `FAIL`.

## Verify

```powershell
go test ./...
go vet ./...
go build ./cmd/quakemap
```
