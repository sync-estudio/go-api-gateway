import { parseListInput, toInteger } from "./form";

export function validateForm(sourceForm) {
  const errors = [];

  if (!sourceForm.proxyHost.trim()) {
    errors.push("Proxy host is required.");
  }

  const proxyPort = toInteger(sourceForm.proxyPort, 0);
  if (proxyPort <= 0 || proxyPort > 65535) {
    errors.push("Proxy port must be between 1 and 65535.");
  }

  const providerSet = new Set();
  for (let i = 0; i < sourceForm.providers.length; i += 1) {
    const provider = sourceForm.providers[i];
    const providerName = provider.name.trim();

    if (!providerName) {
      errors.push(`Provider #${i + 1}: name is required.`);
      continue;
    }

    if (providerSet.has(providerName)) {
      errors.push(`Provider name "${providerName}" is duplicated.`);
    }
    providerSet.add(providerName);

    if ((provider.type || "").trim().toLowerCase() !== "jwks") {
      errors.push(`Provider "${providerName}": only type "jwks" is currently supported.`);
    }

    if (!provider.jwksURL.trim()) {
      errors.push(`Provider "${providerName}": JWKS URL is required.`);
    } else {
      try {
        const parsed = new URL(provider.jwksURL.trim());
        if (!parsed.protocol || !parsed.host) {
          errors.push(`Provider "${providerName}": JWKS URL is invalid.`);
        }
      } catch {
        errors.push(`Provider "${providerName}": JWKS URL is invalid.`);
      }
    }

    if (!provider.refreshInterval.trim()) {
      errors.push(`Provider "${providerName}": refresh interval is required.`);
    }
  }

  const defaultProvider = sourceForm.defaultProvider.trim();
  if (defaultProvider && !providerSet.has(defaultProvider)) {
    errors.push("Default auth provider does not exist in providers list.");
  }

  const usedAliases = new Set();
  for (let i = 0; i < sourceForm.services.length; i += 1) {
    const service = sourceForm.services[i];
    const alias = service.alias.trim();
    const url = service.url.trim();

    if (!url) {
      errors.push(`Service #${i + 1}: URL is required.`);
    } else {
      try {
        const parsed = new URL(url);
        if (!parsed.protocol || !parsed.host) {
          errors.push(`Service #${i + 1}: URL is invalid.`);
        }
      } catch {
        errors.push(`Service #${i + 1}: URL is invalid.`);
      }
    }

    if (!alias) {
      errors.push(`Service #${i + 1}: alias is required.`);
    } else {
      if (!alias.startsWith("/")) {
        errors.push(`Service #${i + 1}: alias must start with '/'.`);
      }

      const normalizedAlias = alias.endsWith("/") ? alias.slice(0, -1) : alias;

      if (normalizedAlias === "/") {
        errors.push(`Service #${i + 1}: alias cannot be '/'.`);
      }

      if (normalizedAlias === "/admin" || normalizedAlias === "/health") {
        errors.push(`Service #${i + 1}: alias "${alias}" is reserved.`);
      }

      if (usedAliases.has(normalizedAlias)) {
        errors.push(`Service alias "${alias}" is duplicated.`);
      }
      usedAliases.add(normalizedAlias);
    }

    const requests = toInteger(service.rateLimitRequests, 0);
    if (requests <= 0) {
      errors.push(`Service #${i + 1}: rate limit requests must be greater than 0.`);
    }

    if (!service.rateLimitWindow.trim()) {
      errors.push(`Service #${i + 1}: rate limit window is required.`);
    }

    if (service.authEnabled) {
      const selectedProvider = service.authProvider.trim() || defaultProvider;
      if (!selectedProvider) {
        errors.push(`Service #${i + 1}: auth enabled but no provider selected and no default provider set.`);
      } else if (!providerSet.has(selectedProvider)) {
        errors.push(`Service #${i + 1}: auth provider "${selectedProvider}" does not exist.`);
      }
    }
  }

  const maxAge = toInteger(sourceForm.corsMaxAge, -1);
  if (maxAge < 0) {
    errors.push("CORS max age must be 0 or greater.");
  }

  const allowedOrigins = parseListInput(sourceForm.corsAllowedOriginsText);
  if (sourceForm.corsAllowCredentials && allowedOrigins.includes("*")) {
    errors.push("CORS cannot allow credentials while allowed origins includes '*'.");
  }

  return errors;
}
