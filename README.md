# go-api-gateway

Minimal Go API gateway that proxies multiple upstream services based on path prefixes.

**What it does**
- Reads service configuration from `config.yaml`.
- Registers each alias as a reverse proxy route.
- Logs requests with method, path, remote address, status, and duration.

**Requirements**
- Go 1.24+

## Configuration

The gateway is configured via `config.yaml`:

```yaml
proxy:
  host: "localhost"
  port: 8080
services:
  - url: "https://service-a.example.com"
    alias: "/warehouse"
  - url: "https://service-b.example.com"
    alias: "/auth"
```

### Configuration Options

| Field | Description |
|-------|-------------|
| `proxy.host` | Host address for the gateway (currently informational) |
| `proxy.port` | Port for the gateway to listen on |
| `services[].url` | Upstream service base URL |
| `services[].alias` | Route prefix that maps to the upstream |

### Environment Variable Overrides

The `PORT` environment variable overrides `proxy.port` from the config file. This is useful for deployment platforms like Railway or Docker:

```bash
PORT=3000 go run cmd/app/main.go
```

## Run

```bash
go run cmd/app/main.go
```

The gateway listens on the port specified in `config.yaml` (defaults to `8080` if not set).

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
  "routes": [
    "/health",
    "/warehouse",
    "/auth"
  ]
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
```yaml
services:
  - url: "https://service-a.example.com"
    alias: "/warehouse"
  - url: "https://service-b.example.com"
    alias: "/auth"
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
"failed to read config.yaml: open config.yaml: no such file or directory"
```

## Project Structure

```
api-gateway/
├── cmd/
│   └── app/
│       └── main.go              # Entry point
├── internal/
│   ├── config/
│   │   └── config.go            # YAML configuration loading
│   ├── handler/
│   │   ├── health.go            # Health check handler
│   │   └── root.go              # Root endpoint handler
│   ├── middleware/
│   │   └── logging.go           # Request logging middleware
│   ├── proxy/
│   │   └── proxy.go             # Reverse proxy creation
│   └── service/
│       └── registry.go          # Service registry
├── config.yaml                  # Gateway configuration
├── go.mod
└── README.md
```

## TODO

### Planned Features

- [ ] **Authentication & Authorization**: Add API key or JWT-based authentication layer
- [ ] **Rate Limiting**: Implement per-route or per-client rate limiting
- [ ] **Circuit Breaker**: Add circuit breaker pattern to handle upstream failures gracefully
- [ ] **Metrics & Monitoring**: Expose Prometheus metrics endpoint for request counts, latencies, and error rates
- [ ] **Request/Response Transformation**: Support for request/response body and header transformations
- [ ] **WebSocket Support**: Enable WebSocket proxying for real-time applications
- [ ] **Configuration Hot-Reload**: Reload service configuration without restarting the gateway
- [ ] **Admin API**: Runtime configuration management (add/remove routes, view stats)
- [ ] **CORS Support**: Configurable CORS headers for browser-based clients
- [ ] **Request Validation**: Schema-based request validation before proxying
- [ ] **Caching**: Response caching for GET requests with configurable TTL
- [ ] **Load Balancing**: Support multiple upstream instances per service with load balancing
- [ ] **Retry Logic**: Automatic retry with exponential backoff for failed upstream requests
- [ ] **Request Tracing**: Distributed tracing support (OpenTelemetry/Jaeger)
- [ ] **TLS/SSL Configuration**: Custom TLS settings for upstream connections

## Notes

- Configuration is loaded from `config.yaml` at startup.
- The `PORT` environment variable overrides the port in the config file.
- All requests are logged with method, path, remote address, upstream target, status, duration, and headers.
