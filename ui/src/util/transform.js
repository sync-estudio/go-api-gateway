import { createDefaultForm, listToInput, parseListInput, toInteger } from "./form";

export function toForm(payload) {
  const nextForm = createDefaultForm();

  nextForm.proxyHost = payload?.proxy?.host || "localhost";
  nextForm.proxyPort = toInteger(payload?.proxy?.port, 8080);

  nextForm.corsEnabled = Boolean(payload?.cors?.enabled);
  nextForm.corsAllowedOriginsText = listToInput(payload?.cors?.allowed_origins || ["*"]);
  nextForm.corsAllowedMethods = Array.isArray(payload?.cors?.allowed_methods)
    ? payload.cors.allowed_methods
    : [...nextForm.corsAllowedMethods];
  nextForm.corsAllowedHeadersText = listToInput(payload?.cors?.allowed_headers || []);
  nextForm.corsExposedHeadersText = listToInput(payload?.cors?.exposed_headers || []);
  nextForm.corsAllowCredentials = Boolean(payload?.cors?.allow_credentials);
  nextForm.corsMaxAge = toInteger(payload?.cors?.max_age, 86400);

  nextForm.defaultProvider = payload?.auth?.default_provider || "";
  nextForm.providers = Object.entries(payload?.auth?.providers || {}).map(([name, provider]) => ({
    name,
    type: provider?.type || "jwks",
    jwksURL: provider?.jwks_url || "",
    refreshInterval: provider?.refresh_interval || "1h",
    issuer: provider?.issuer || "",
  }));

  nextForm.services = Array.isArray(payload?.services)
    ? payload.services.map((service) => ({
        url: service?.url || "",
        alias: service?.alias || "",
        authEnabled: Boolean(service?.auth?.enabled),
        authProvider: service?.auth?.provider || "",
        rateLimitRequests: toInteger(service?.rate_limit?.requests, 200),
        rateLimitWindow: service?.rate_limit?.window || "1m",
      }))
    : [];

  return nextForm;
}

export function buildPayloadFromForm(sourceForm) {
  const providers = {};
  for (const provider of sourceForm.providers) {
    const name = provider.name.trim();
    if (!name) {
      continue;
    }

    providers[name] = {
      type: (provider.type || "jwks").trim(),
      jwks_url: provider.jwksURL.trim(),
      refresh_interval: provider.refreshInterval.trim() || "1h",
      issuer: provider.issuer.trim(),
    };
  }

  const services = sourceForm.services.map((service) => ({
    url: service.url.trim(),
    alias: service.alias.trim(),
    auth: {
      enabled: Boolean(service.authEnabled),
      provider: service.authProvider.trim(),
    },
    rate_limit: {
      requests: toInteger(service.rateLimitRequests, 200),
      window: (service.rateLimitWindow || "").trim() || "1m",
    },
  }));

  return {
    proxy: {
      host: sourceForm.proxyHost.trim() || "localhost",
      port: toInteger(sourceForm.proxyPort, 8080),
    },
    auth: {
      default_provider: sourceForm.defaultProvider.trim(),
      providers,
    },
    cors: {
      enabled: Boolean(sourceForm.corsEnabled),
      allowed_origins: parseListInput(sourceForm.corsAllowedOriginsText),
      allowed_methods: sourceForm.corsAllowedMethods.map((method) => method.trim()).filter(Boolean),
      allowed_headers: parseListInput(sourceForm.corsAllowedHeadersText),
      exposed_headers: parseListInput(sourceForm.corsExposedHeadersText),
      allow_credentials: Boolean(sourceForm.corsAllowCredentials),
      max_age: toInteger(sourceForm.corsMaxAge, 86400),
    },
    services,
  };
}

export function normalizeForCompare(payload) {
  const copy = JSON.parse(JSON.stringify(payload || {}));
  const providers = copy?.auth?.providers;

  if (providers && typeof providers === "object" && !Array.isArray(providers)) {
    const ordered = {};
    Object.keys(providers)
      .sort((a, b) => a.localeCompare(b))
      .forEach((key) => {
        ordered[key] = providers[key];
      });
    copy.auth.providers = ordered;
  }

  return copy;
}

export function toCanonical(payload) {
  return JSON.stringify(normalizeForCompare(payload));
}
