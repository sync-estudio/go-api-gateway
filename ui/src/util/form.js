export function createDefaultForm() {
  return {
    proxyHost: "localhost",
    proxyPort: 8080,
    corsEnabled: false,
    corsAllowedOriginsText: "*",
    corsAllowedMethods: ["GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"],
    corsAllowedHeadersText: "Content-Type\nAuthorization",
    corsExposedHeadersText: "",
    corsAllowCredentials: false,
    corsMaxAge: 86400,
    defaultProvider: "",
    providers: [],
    services: [],
  };
}

export function createEmptyProvider() {
  return {
    name: "",
    type: "jwks",
    jwksURL: "",
    refreshInterval: "1h",
    issuer: "",
  };
}

export function createEmptyService() {
  return {
    url: "",
    alias: "",
    authEnabled: false,
    authProvider: "",
    rateLimitRequests: 200,
    rateLimitWindow: "1m",
  };
}

export function parseListInput(value) {
  return String(value || "")
    .split(/\r?\n|,/)
    .map((item) => item.trim())
    .filter(Boolean);
}

export function listToInput(value) {
  if (!Array.isArray(value)) {
    return "";
  }

  return value.join("\n");
}

export function toInteger(value, fallback = 0) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    return fallback;
  }

  return Math.trunc(parsed);
}
