# go-api-gateway

Minimal Go API gateway that proxies multiple upstream services based on path prefixes.

**What it does**

- Reads service configuration from `config.json`.
- Registers each alias as a reverse proxy route.
- Logs requests with method, path, remote address, status, and duration.
- Includes `/admin` UI for live config updates without restart.

**Requirements**

- Go 1.24+
- Node 20+ (only if developing the `/admin` UI locally)

## Configuration

The gateway is configured via `config.json`:

```json
{
  "proxy": {
    "host": "localhost",
    "port": 8080
  },
  "services": [
    {
      "url": "https://service-a.example.com",
      "alias": "/warehouse"
    },
    {
      "url": "https://service-b.example.com",
      "alias": "/auth"
    }
  ]
}
```

### Configuration Options

| Field                    | Description                                            |
| ------------------------ | ------------------------------------------------------ |
| `proxy.host`             | Host address for the gateway (currently informational) |
| `proxy.port`             | Port for the gateway to listen on                      |
| `cors.enabled`           | Enable CORS middleware                                 |
| `cors.allowed_origins`   | List of allowed origins (supports "\*" for all)        |
| `cors.allowed_methods`   | List of allowed HTTP methods                           |
| `cors.allowed_headers`   | List of allowed request headers                        |
| `cors.exposed_headers`   | List of headers exposed to the browser                 |
| `cors.allow_credentials` | Allow credentials (cookies, auth headers)              |
| `cors.max_age`           | Preflight cache duration in seconds                    |
| `services[].url`         | Upstream service base URL                              |
| `services[].alias`       | Route prefix that maps to the upstream                 |

### Environment Variable Overrides

The `PORT` environment variable overrides `proxy.port` from the config file. You can also override config location with `CONFIG_PATH`.

```bash
PORT=3000 CONFIG_PATH=./config.json go run cmd/app/main.go
```

### Admin UI (Live Config)

The gateway includes a Svelte + Tailwind admin dashboard at `/admin` with form-based config editing (plus optional advanced JSON view). Changes are applied live without restart.

Set these env vars to enable admin login:

```bash
ADMIN_EMAIL=admin@example.com
ADMIN_PASSWORD=super-secret-temp-password
ADMIN_SESSION_SECRET=replace-with-a-long-random-secret
# optional (default 30m)
ADMIN_SESSION_TTL=30m
```

If any of `ADMIN_EMAIL`, `ADMIN_PASSWORD`, or `ADMIN_SESSION_SECRET` is missing, admin login/API stays disabled.

UI development commands:

```bash
cd ui
npm install
npm run dev
```

UI production build:

```bash
cd ui
npm run build
```

The Docker build compiles the UI automatically and copies `ui/dist` into the final runtime image.

When running with Docker Compose, live config edits are persisted to `./config/config.json` on the host.

Makefile shortcuts:

```bash
make ui-install
make ui-dev
make ui-build
make run
make test
make build
make docker-build
make up
make down
make logs
```

### Redis (Rate Limiter)

The gateway requires Redis for rate limiting. Redis configuration is resolved in this order:

1. `REDIS_PRIVATE_URL` (recommended on Railway private networking)
2. `REDIS_URL`
3. `REDIS_ADDR`
4. `REDISHOST` + `REDISPORT` (+ optional `REDISUSER` / `REDISPASSWORD`)

For Railway private networking, set:

```bash
REDIS_PRIVATE_URL=redis://default:<password>@<redis-service>.railway.internal:6379
```

Avoid using `*.proxy.rlwy.net` if you want traffic to stay on private networking.

If using `REDISHOST`/`REDISPORT`, set:

```bash
REDISHOST=redis.railway.internal
REDISPORT=6379
REDISUSER=default
REDISPASSWORD=<password>
```

`REDISHOST` must be a hostname only (no `http://` or `https://`).

## Run

```bash
go run cmd/app/main.go
```

The gateway listens on the port specified in `config.json` (defaults to `8080` if not set).

## Routing Behavior

- Requests to `/alias/...` are proxied to the corresponding upstream.
- Requests to `/alias` redirect to `/alias/`.
- The alias prefix is stripped before proxying.

**Example**

With the sample config above:

- `GET /warehouse/items` -> `https://service-a.example.com/items`
- `GET /auth/login` -> `https://service-b.example.com/login`

## API Reference

### Root Endpoint

**GET /**

Returns gateway information and list of available routes.

**Response**: 200 OK

```json
{
  "message": "API Gateway",
  "routes": ["/health", "/warehouse", "/auth"]
}
```

### Health Check

**GET /health**

Check gateway service health status.

**Response**: 200 OK

```json
{
  "status": "healthy",
  "service": "api-gateway"
}
```

### Proxy Routes

**{METHOD} /{alias}/{path}**

Proxies requests to configured upstream services based on the alias prefix.

**Behavior**:

- The alias prefix is stripped before forwarding to upstream
- All HTTP methods are supported (GET, POST, PUT, DELETE, PATCH, etc.)
- Request headers are forwarded (Authorization, Cookie, etc.)
- Query parameters are preserved
- Request and response bodies are proxied as-is

**Examples**:

Given configuration:

```json
{
  "services": [
    { "url": "https://service-a.example.com", "alias": "/warehouse" },
    { "url": "https://service-b.example.com", "alias": "/auth" }
  ]
}
```

**Request**: `GET /warehouse/items?status=active`

- **Proxied to**: `https://service-a.example.com/items?status=active`
- **Response**: Returns upstream service response

**Request**: `POST /auth/login` with body

- **Proxied to**: `https://service-b.example.com/login`
- **Response**: Returns upstream service response

**Request**: `GET /warehouse` (without trailing slash)

- **Response**: 301 redirect to `/warehouse/`

## Error Handling

### Common Status Codes

- **200 OK**: Successful request
- **301 Moved Permanently**: Redirect from bare alias path to path with trailing slash
- **404 Not Found**: Route not registered or upstream returns 404
- **502 Bad Gateway**: Upstream service is unreachable or returns an error
- **500 Internal Server Error**: Gateway internal error

### Error Scenarios

**Unknown route**:

```
GET /unknown/path
→ 404 Not Found
```

**Upstream service down**:

```
GET /warehouse/items
→ 502 Bad Gateway (if service-a.example.com is unreachable)
```

**Missing or invalid configuration**:

```
Server fails to start with error:
"failed to read config.json: open config.json: no such file or directory"
```

## Project Structure

```
api-gateway/
├── cmd/
│   └── app/
│       └── main.go              # Entry point
├── internal/
│   ├── config/
│   │   └── config.go            # JSON/YAML configuration loading + validation
│   ├── admin/
│   │   └── handler.go           # Admin auth/session + config API + static UI serving
│   ├── handler/
│   │   ├── health.go            # Health check handler
│   │   └── root.go              # Root endpoint handler
│   ├── middleware/
│   │   └── logging.go           # Request logging middleware
│   ├── runtime/
│   │   └── manager.go           # Hot-swappable runtime config manager
│   ├── proxy/
│   │   └── proxy.go             # Reverse proxy creation
│   └── service/
│       └── registry.go          # Service registry
├── ui/                          # Svelte + Tailwind admin frontend
│   ├── src/
│   ├── package.json
│   └── vite.config.js
├── config.json                  # Gateway configuration
├── go.mod
└── README.md
```

## TODO

### Planned Features

- [x] **Authentication & Authorization**: Add API key or JWT-based authentication layer
- [x] **Rate Limiting**: Implement per-route or per-client rate limiting
- [ ] **Circuit Breaker**: Add circuit breaker pattern to handle upstream failures gracefully
- [ ] **Metrics & Monitoring**: Expose Prometheus metrics endpoint for request counts, latencies, and error rates
- [ ] **Request/Response Transformation**: Support for request/response body and header transformations
- [ ] **WebSocket Support**: Enable WebSocket proxying for real-time applications
- [ ] **Configuration Hot-Reload**: Reload service configuration without restarting the gateway
- [ ] **Admin API**: Runtime configuration management (add/remove routes, view stats)
- [x] **CORS Support**: Configurable CORS headers for browser-based clients
- [ ] **Request Validation**: Schema-based request validation before proxying
- [ ] **Caching**: Response caching for GET requests with configurable TTL
- [ ] **Load Balancing**: Support multiple upstream instances per service with load balancing
- [ ] **Retry Logic**: Automatic retry with exponential backoff for failed upstream requests
- [ ] **Request Tracing**: Distributed tracing support (OpenTelemetry/Jaeger)
- [ ] **TLS/SSL Configuration**: Custom TLS settings for upstream connections

## Notes

- Configuration is loaded from `config.json` at startup.
- If `config.json` does not exist and `config.yaml` exists, the gateway loads YAML once and migrates to JSON.
- The `PORT` environment variable overrides the port in the config file.
- All requests are logged with method, path, remote address, upstream target, status, duration, and headers.
