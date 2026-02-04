# go-api-gateway

Minimal Go API gateway that proxies multiple upstream services based on path prefixes.

**What it does**
- Reads service URLs and route aliases from environment variables.
- Registers each alias as a reverse proxy route.
- Logs requests with method, path, remote address, status, and duration.

**Requirements**
- Go 1.24+

**Configuration**
Set the following environment variables (comma-separated lists must be the same length):
- `PORT`: port for the gateway to listen on
- `SERVICES_URL`: upstream base URLs
- `SERVICES_ALIASES`: route prefixes for each upstream

Example `.env`:
```env
PORT=8082
SERVICES_URL="https://service-a.example.com,https://service-b.example.com"
SERVICES_ALIASES="/warehouse,/auth"
```

**Run**
```bash
go run cmd/app/main.go
```
The gateway listens on `:PORT` (defaults to `:8080` if `PORT` is not set).

**Routing behavior**
- Requests to `/alias/...` are proxied to the corresponding upstream.
- Requests to `/alias` redirect to `/alias/`.
- The alias prefix is stripped before proxying.

**Example**
With the sample `.env` above:
- `GET /warehouse/items` -> `https://service-a.example.com/items`
- `GET /auth/login` -> `https://service-b.example.com/login`

**Notes**
- The `.env` file is loaded automatically (via `godotenv`) when present.
- If you add or remove services, update both env lists to keep them aligned.
