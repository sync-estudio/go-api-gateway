# syntax=docker/dockerfile:1

FROM node:22-alpine AS ui-build
WORKDIR /ui

COPY ui/package*.json ./
RUN npm ci

COPY ui/ ./
RUN npm run build

FROM golang:1.25-alpine AS build
WORKDIR /src

# DEPS
COPY go.mod go.sum ./
RUN go mod download

# BINARY
COPY . .
COPY --from=ui-build /ui/dist ./ui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api-gateway ./cmd/app

# RUNTIME
FROM alpine:3.22
RUN apk add --no-cache ca-certificates
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app

COPY --from=build /bin/api-gateway /app/api-gateway
COPY --chown=appuser:appgroup config.json /app/config.json
COPY --from=ui-build --chown=appuser:appgroup /ui/dist /app/ui/dist
RUN mkdir -p /app/config && chown -R appuser:appgroup /app

EXPOSE 8080
USER appuser
ENTRYPOINT ["/app/api-gateway"]
