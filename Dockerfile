# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS build
WORKDIR /src

# DEPS
COPY go.mod go.sum ./
RUN go mod download

# BINARY
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api-gateway ./cmd/app

# RUNTIME
FROM alpine:3.22
RUN apk add --no-cache ca-certificates
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app

COPY --from=build /bin/api-gateway /app/api-gateway
COPY config.yaml /app/config.yaml

EXPOSE 8080
USER appuser
ENTRYPOINT ["/app/api-gateway"]
