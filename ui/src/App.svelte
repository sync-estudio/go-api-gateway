<script>
  import { onMount } from "svelte";

  const METHOD_OPTIONS = ["GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"];

  let loadingSession = true;
  let enabled = false;
  let authenticated = false;

  let email = "";
  let password = "";
  let accountEmail = "";

  let busy = false;
  let status = "";
  let statusIsError = false;

  let activeTab = "dashboard";
  let validationErrors = [];

  let form = createDefaultForm();
  let jsonDraft = "";

  let baselineFormCanonical = "";
  let baselineJsonCanonical = "";

  let providerNames = [];
  let dashboardDirty = false;
  let jsonDirty = false;

  $: providerNames = form.providers.map((provider) => provider.name.trim()).filter(Boolean);

  $: dashboardDirty = (() => {
    try {
      return toCanonical(buildPayloadFromForm(form)) !== baselineFormCanonical;
    } catch {
      return true;
    }
  })();

  $: jsonDirty = (() => {
    try {
      return toCanonical(JSON.parse(jsonDraft || "{}")) !== baselineJsonCanonical;
    } catch {
      return true;
    }
  })();

  function createDefaultForm() {
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

  function createEmptyProvider() {
    return {
      name: "",
      type: "jwks",
      jwksURL: "",
      refreshInterval: "1h",
      issuer: "",
    };
  }

  function createEmptyService() {
    return {
      url: "",
      alias: "",
      authEnabled: false,
      authProvider: "",
      rateLimitRequests: 200,
      rateLimitWindow: "1m",
    };
  }

  function parseListInput(value) {
    return String(value || "")
      .split(/\r?\n|,/) 
      .map((item) => item.trim())
      .filter(Boolean);
  }

  function listToInput(value) {
    if (!Array.isArray(value)) {
      return "";
    }
    return value.join("\n");
  }

  function toInteger(value, fallback = 0) {
    const parsed = Number(value);
    if (!Number.isFinite(parsed)) {
      return fallback;
    }
    return Math.trunc(parsed);
  }

  function toForm(payload) {
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

  function buildPayloadFromForm(sourceForm) {
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

  function normalizeForCompare(payload) {
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

  function toCanonical(payload) {
    return JSON.stringify(normalizeForCompare(payload));
  }

  function applyLoadedConfig(payload) {
    form = toForm(payload);
    jsonDraft = JSON.stringify(payload, null, 2);

    baselineFormCanonical = toCanonical(buildPayloadFromForm(form));
    baselineJsonCanonical = toCanonical(payload);
    validationErrors = [];
  }

  function validateForm(sourceForm) {
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
        errors.push(`Provider name \"${providerName}\" is duplicated.`);
      }
      providerSet.add(providerName);

      if ((provider.type || "").trim().toLowerCase() !== "jwks") {
        errors.push(`Provider \"${providerName}\": only type \"jwks\" is currently supported.`);
      }

      if (!provider.jwksURL.trim()) {
        errors.push(`Provider \"${providerName}\": JWKS URL is required.`);
      } else {
        try {
          const parsed = new URL(provider.jwksURL.trim());
          if (!parsed.protocol || !parsed.host) {
            errors.push(`Provider \"${providerName}\": JWKS URL is invalid.`);
          }
        } catch {
          errors.push(`Provider \"${providerName}\": JWKS URL is invalid.`);
        }
      }

      if (!provider.refreshInterval.trim()) {
        errors.push(`Provider \"${providerName}\": refresh interval is required.`);
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
          errors.push(`Service #${i + 1}: alias \"${alias}\" is reserved.`);
        }

        if (usedAliases.has(normalizedAlias)) {
          errors.push(`Service alias \"${alias}\" is duplicated.`);
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
          errors.push(`Service #${i + 1}: auth provider \"${selectedProvider}\" does not exist.`);
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

  function setStatus(message, isError = false) {
    status = message || "";
    statusIsError = isError;
  }

  async function requestJSON(path, options = {}) {
    const response = await fetch(path, {
      credentials: "same-origin",
      ...options,
      headers: {
        "Content-Type": "application/json",
        ...(options.headers || {}),
      },
    });

    const payload = await response.json().catch(() => ({}));

    if (!response.ok) {
      throw new Error(payload.error || "request failed");
    }

    return payload;
  }

  async function checkSession() {
    loadingSession = true;
    try {
      const payload = await requestJSON("/admin/api/session", { method: "GET" });

      enabled = Boolean(payload.enabled);
      authenticated = Boolean(payload.authenticated);
      accountEmail = payload.email || "";

      if (!enabled) {
        setStatus(
          "Admin UI is disabled. Set ADMIN_EMAIL, ADMIN_PASSWORD, and ADMIN_SESSION_SECRET.",
          true,
        );
        return;
      }

      if (authenticated) {
        await loadConfig();
      } else {
        setStatus("Sign in to manage gateway configuration.");
      }
    } catch (error) {
      setStatus(error.message, true);
    } finally {
      loadingSession = false;
    }
  }

  async function loadConfig() {
    busy = true;
    try {
      const payload = await requestJSON("/admin/api/config", { method: "GET" });
      applyLoadedConfig(payload);
      setStatus("Config loaded.");
    } catch (error) {
      setStatus(error.message, true);
    } finally {
      busy = false;
    }
  }

  async function login(event) {
    event.preventDefault();
    busy = true;
    try {
      await requestJSON("/admin/api/login", {
        method: "POST",
        body: JSON.stringify({ email, password }),
      });

      email = "";
      password = "";
      authenticated = true;
      await checkSession();
    } catch (error) {
      setStatus(error.message, true);
    } finally {
      busy = false;
    }
  }

  async function logout() {
    busy = true;
    try {
      await requestJSON("/admin/api/logout", {
        method: "POST",
      });
      authenticated = false;
      accountEmail = "";
      form = createDefaultForm();
      jsonDraft = "";
      validationErrors = [];
      baselineFormCanonical = "";
      baselineJsonCanonical = "";
      setStatus("Logged out.");
      await checkSession();
    } catch (error) {
      setStatus(error.message, true);
    } finally {
      busy = false;
    }
  }

  async function saveDashboard() {
    const errors = validateForm(form);
    validationErrors = errors;

    if (errors.length > 0) {
      setStatus(errors[0], true);
      return;
    }

    const payload = buildPayloadFromForm(form);

    busy = true;
    try {
      await requestJSON("/admin/api/config", {
        method: "PUT",
        body: JSON.stringify(payload),
      });

      const latest = await requestJSON("/admin/api/config", { method: "GET" });
      applyLoadedConfig(latest);
      setStatus("Dashboard changes saved and applied.");
    } catch (error) {
      setStatus(error.message, true);
    } finally {
      busy = false;
    }
  }

  async function saveJSON() {
    busy = true;
    try {
      const parsed = JSON.parse(jsonDraft);

      await requestJSON("/admin/api/config", {
        method: "PUT",
        body: JSON.stringify(parsed),
      });

      const latest = await requestJSON("/admin/api/config", { method: "GET" });
      applyLoadedConfig(latest);
      setStatus("Advanced JSON changes saved and applied.");
    } catch (error) {
      setStatus(error.message, true);
    } finally {
      busy = false;
    }
  }

  function syncJsonFromForm() {
    const errors = validateForm(form);
    validationErrors = errors;

    if (errors.length > 0) {
      setStatus(errors[0], true);
      return;
    }

    jsonDraft = JSON.stringify(buildPayloadFromForm(form), null, 2);
    activeTab = "json";
    setStatus("Advanced JSON updated from dashboard form.");
  }

  function syncFormFromJson() {
    try {
      const parsed = JSON.parse(jsonDraft);
      form = toForm(parsed);
      validationErrors = [];
      activeTab = "dashboard";
      setStatus("Dashboard form updated from advanced JSON.");
    } catch (error) {
      setStatus(`Invalid JSON: ${error.message}`, true);
    }
  }

  function addProvider() {
    form = {
      ...form,
      providers: [...form.providers, createEmptyProvider()],
    };
  }

  function removeProvider(index) {
    const removedName = form.providers[index]?.name?.trim() || "";
    const providers = form.providers.filter((_, idx) => idx !== index);

    const services = form.services.map((service) => {
      if (service.authProvider.trim() === removedName) {
        return {
          ...service,
          authProvider: "",
        };
      }
      return service;
    });

    form = {
      ...form,
      providers,
      services,
      defaultProvider: form.defaultProvider.trim() === removedName ? "" : form.defaultProvider,
    };
  }

  function addService() {
    form = {
      ...form,
      services: [...form.services, createEmptyService()],
    };
  }

  function removeService(index) {
    form = {
      ...form,
      services: form.services.filter((_, idx) => idx !== index),
    };
  }

  onMount(checkSession);
</script>

<main class="min-h-screen bg-slate-100 p-4 text-slate-900 sm:p-8">
  <section class="mx-auto w-full max-w-6xl rounded-2xl border border-slate-200 bg-white shadow-xl">
    <header class="border-b border-slate-200 px-6 py-5 sm:px-8">
      <div class="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 class="text-xl font-semibold tracking-tight sm:text-2xl">Gateway Admin Dashboard</h1>
          <p class="mt-1 text-sm text-slate-500">Manage gateway config with guided forms and live apply.</p>
        </div>

        {#if enabled && authenticated}
          <div class="flex items-center gap-3">
            <span class="rounded-full bg-teal-50 px-3 py-1 text-xs font-medium text-teal-700">{accountEmail}</span>
            <button
              type="button"
              class="rounded-lg bg-rose-600 px-3 py-2 text-sm font-semibold text-white transition hover:bg-rose-700 disabled:cursor-not-allowed disabled:opacity-70"
              on:click={logout}
              disabled={busy}
            >
              Logout
            </button>
          </div>
        {/if}
      </div>
    </header>

    <div class="px-6 py-6 sm:px-8">
      {#if loadingSession}
        <div class="rounded-xl border border-slate-200 bg-slate-50 p-6 text-sm text-slate-600">Checking session...</div>
      {:else if !enabled}
        <div class="rounded-xl border border-amber-300 bg-amber-50 p-6 text-sm text-amber-800">
          Admin UI login is disabled. Set `ADMIN_EMAIL`, `ADMIN_PASSWORD`, and `ADMIN_SESSION_SECRET`.
        </div>
      {:else if !authenticated}
        <form class="mx-auto max-w-md space-y-4" on:submit={login}>
          <div>
            <label class="mb-1 block text-sm font-medium text-slate-600" for="email">Admin email</label>
            <input
              id="email"
              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
              type="email"
              autocomplete="username"
              bind:value={email}
              required
            />
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium text-slate-600" for="password">Admin password</label>
            <input
              id="password"
              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
              type="password"
              autocomplete="current-password"
              bind:value={password}
              required
            />
          </div>

          <button
            type="submit"
            class="w-full rounded-lg bg-gradient-to-r from-teal-700 to-cyan-600 px-4 py-2.5 text-sm font-semibold text-white transition hover:from-teal-800 hover:to-cyan-700 disabled:cursor-not-allowed disabled:opacity-70"
            disabled={busy}
          >
            {busy ? "Signing in..." : "Sign in"}
          </button>
        </form>
      {:else}
        <div class="space-y-5">
          <div class="flex flex-wrap gap-2">
            <button
              type="button"
              class="rounded-lg border px-3 py-1.5 text-sm font-semibold transition"
              class:border-cyan-600={activeTab === "dashboard"}
              class:bg-cyan-50={activeTab === "dashboard"}
              class:text-cyan-700={activeTab === "dashboard"}
              class:border-slate-300={activeTab !== "dashboard"}
              class:bg-white={activeTab !== "dashboard"}
              class:text-slate-700={activeTab !== "dashboard"}
              on:click={() => {
                activeTab = "dashboard";
              }}
            >
              Dashboard {dashboardDirty ? "*" : ""}
            </button>
            <button
              type="button"
              class="rounded-lg border px-3 py-1.5 text-sm font-semibold transition"
              class:border-cyan-600={activeTab === "json"}
              class:bg-cyan-50={activeTab === "json"}
              class:text-cyan-700={activeTab === "json"}
              class:border-slate-300={activeTab !== "json"}
              class:bg-white={activeTab !== "json"}
              class:text-slate-700={activeTab !== "json"}
              on:click={() => {
                activeTab = "json";
              }}
            >
              Advanced JSON {jsonDirty ? "*" : ""}
            </button>
          </div>

          {#if validationErrors.length > 0 && activeTab === "dashboard"}
            <div class="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3">
              <p class="text-sm font-semibold text-rose-700">Please fix the following:</p>
              <ul class="mt-2 list-disc space-y-1 pl-5 text-sm text-rose-700">
                {#each validationErrors as errorMessage}
                  <li>{errorMessage}</li>
                {/each}
              </ul>
            </div>
          {/if}

          {#if activeTab === "dashboard"}
            <div class="space-y-5">
              <section class="rounded-xl border border-slate-200 p-4">
                <h2 class="text-base font-semibold text-slate-800">Gateway</h2>
                <p class="mt-1 text-xs text-slate-500">Core gateway host and port settings.</p>

                <div class="mt-4 grid gap-4 sm:grid-cols-2">
                  <div>
                    <label class="mb-1 block text-sm font-medium text-slate-600" for="proxyHost">Proxy host</label>
                    <input
                      id="proxyHost"
                      class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                      type="text"
                      bind:value={form.proxyHost}
                    />
                  </div>

                  <div>
                    <label class="mb-1 block text-sm font-medium text-slate-600" for="proxyPort">Proxy port</label>
                    <input
                      id="proxyPort"
                      class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                      type="number"
                      min="1"
                      max="65535"
                      bind:value={form.proxyPort}
                    />
                    <p class="mt-1 text-xs text-amber-700">Changing port still requires process/container restart.</p>
                  </div>
                </div>
              </section>

              <section class="rounded-xl border border-slate-200 p-4">
                <h2 class="text-base font-semibold text-slate-800">CORS</h2>
                <p class="mt-1 text-xs text-slate-500">Browser access controls for cross-origin requests.</p>

                <div class="mt-4 space-y-4">
                  <label class="flex items-center gap-2 text-sm text-slate-700">
                    <input type="checkbox" class="h-4 w-4" bind:checked={form.corsEnabled} />
                    Enable CORS
                  </label>

                  <div>
                    <label class="mb-1 block text-sm font-medium text-slate-600" for="corsAllowedOrigins">Allowed origins (one per line)</label>
                    <textarea
                      id="corsAllowedOrigins"
                      class="min-h-[88px] w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                      bind:value={form.corsAllowedOriginsText}
                    ></textarea>
                  </div>

                  <div>
                    <p class="mb-2 block text-sm font-medium text-slate-600">Allowed methods</p>
                    <div class="flex flex-wrap gap-3">
                      {#each METHOD_OPTIONS as method}
                        <label class="flex items-center gap-2 rounded-md border border-slate-300 px-2.5 py-1.5 text-sm text-slate-700">
                          <input type="checkbox" value={method} bind:group={form.corsAllowedMethods} />
                          {method}
                        </label>
                      {/each}
                    </div>
                  </div>

                  <div class="grid gap-4 sm:grid-cols-2">
                    <div>
                      <label class="mb-1 block text-sm font-medium text-slate-600" for="corsAllowedHeaders">Allowed headers (one per line)</label>
                      <textarea
                        id="corsAllowedHeaders"
                        class="min-h-[88px] w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                        bind:value={form.corsAllowedHeadersText}
                      ></textarea>
                    </div>

                    <div>
                      <label class="mb-1 block text-sm font-medium text-slate-600" for="corsExposedHeaders">Exposed headers (one per line)</label>
                      <textarea
                        id="corsExposedHeaders"
                        class="min-h-[88px] w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                        bind:value={form.corsExposedHeadersText}
                      ></textarea>
                    </div>
                  </div>

                  <div class="grid gap-4 sm:grid-cols-2">
                    <label class="flex items-center gap-2 text-sm text-slate-700">
                      <input type="checkbox" class="h-4 w-4" bind:checked={form.corsAllowCredentials} />
                      Allow credentials
                    </label>

                    <div>
                      <label class="mb-1 block text-sm font-medium text-slate-600" for="corsMaxAge">Preflight max age (seconds)</label>
                      <input
                        id="corsMaxAge"
                        class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                        type="number"
                        min="0"
                        bind:value={form.corsMaxAge}
                      />
                    </div>
                  </div>
                </div>
              </section>

              <section class="rounded-xl border border-slate-200 p-4">
                <div class="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <h2 class="text-base font-semibold text-slate-800">Auth Providers</h2>
                    <p class="mt-1 text-xs text-slate-500">Define available providers and select the default provider.</p>
                  </div>

                  <button
                    type="button"
                    class="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm font-semibold text-slate-700 transition hover:bg-slate-50"
                    on:click={addProvider}
                  >
                    Add provider
                  </button>
                </div>

                <div class="mt-4">
                  <label class="mb-1 block text-sm font-medium text-slate-600" for="defaultProvider">Default provider</label>
                  <select
                    id="defaultProvider"
                    class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                    bind:value={form.defaultProvider}
                  >
                    <option value="">None</option>
                    {#each providerNames as providerName}
                      <option value={providerName}>{providerName}</option>
                    {/each}
                  </select>
                </div>

                {#if form.providers.length === 0}
                  <p class="mt-4 text-sm text-slate-500">No providers yet.</p>
                {:else}
                  <div class="mt-4 space-y-4">
                    {#each form.providers as provider, index}
                      <article class="rounded-lg border border-slate-200 bg-slate-50 p-3">
                        <div class="mb-3 flex items-center justify-between gap-3">
                          <h3 class="text-sm font-semibold text-slate-700">Provider #{index + 1}</h3>
                          <button
                            type="button"
                            class="rounded-md bg-rose-600 px-2.5 py-1 text-xs font-semibold text-white transition hover:bg-rose-700"
                            on:click={() => {
                              removeProvider(index);
                            }}
                          >
                            Remove
                          </button>
                        </div>

                        <div class="grid gap-3 sm:grid-cols-2">
                          <div>
                            <label class="mb-1 block text-xs font-medium text-slate-600" for={`provider-${index}-name`}>Name</label>
                            <input
                              id={`provider-${index}-name`}
                              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                              type="text"
                              bind:value={provider.name}
                            />
                          </div>

                          <div>
                            <label class="mb-1 block text-xs font-medium text-slate-600" for={`provider-${index}-type`}>Type</label>
                            <select
                              id={`provider-${index}-type`}
                              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                              bind:value={provider.type}
                            >
                              <option value="jwks">jwks</option>
                            </select>
                          </div>

                          <div class="sm:col-span-2">
                            <label class="mb-1 block text-xs font-medium text-slate-600" for={`provider-${index}-jwks`}>JWKS URL</label>
                            <input
                              id={`provider-${index}-jwks`}
                              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                              type="text"
                              bind:value={provider.jwksURL}
                            />
                          </div>

                          <div>
                            <label class="mb-1 block text-xs font-medium text-slate-600" for={`provider-${index}-refresh`}>Refresh interval</label>
                            <input
                              id={`provider-${index}-refresh`}
                              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                              type="text"
                              placeholder="1h"
                              bind:value={provider.refreshInterval}
                            />
                          </div>

                          <div>
                            <label class="mb-1 block text-xs font-medium text-slate-600" for={`provider-${index}-issuer`}>Issuer (optional)</label>
                            <input
                              id={`provider-${index}-issuer`}
                              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                              type="text"
                              bind:value={provider.issuer}
                            />
                          </div>
                        </div>
                      </article>
                    {/each}
                  </div>
                {/if}
              </section>

              <section class="rounded-xl border border-slate-200 p-4">
                <div class="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <h2 class="text-base font-semibold text-slate-800">Services</h2>
                    <p class="mt-1 text-xs text-slate-500">Configure proxied aliases, upstream URLs, auth, and rate limits.</p>
                  </div>

                  <button
                    type="button"
                    class="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm font-semibold text-slate-700 transition hover:bg-slate-50"
                    on:click={addService}
                  >
                    Add service
                  </button>
                </div>

                {#if form.services.length === 0}
                  <p class="mt-4 text-sm text-slate-500">No services configured yet.</p>
                {:else}
                  <div class="mt-4 space-y-4">
                    {#each form.services as service, index}
                      <article class="rounded-lg border border-slate-200 bg-slate-50 p-3">
                        <div class="mb-3 flex items-center justify-between gap-3">
                          <h3 class="text-sm font-semibold text-slate-700">Service #{index + 1}</h3>
                          <button
                            type="button"
                            class="rounded-md bg-rose-600 px-2.5 py-1 text-xs font-semibold text-white transition hover:bg-rose-700"
                            on:click={() => {
                              removeService(index);
                            }}
                          >
                            Remove
                          </button>
                        </div>

                        <div class="grid gap-3 sm:grid-cols-2">
                          <div class="sm:col-span-2">
                            <label class="mb-1 block text-xs font-medium text-slate-600" for={`service-${index}-url`}>Upstream URL</label>
                            <input
                              id={`service-${index}-url`}
                              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                              type="text"
                              bind:value={service.url}
                            />
                          </div>

                          <div>
                            <label class="mb-1 block text-xs font-medium text-slate-600" for={`service-${index}-alias`}>Alias</label>
                            <input
                              id={`service-${index}-alias`}
                              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                              type="text"
                              placeholder="/warehouse"
                              bind:value={service.alias}
                            />
                          </div>

                          <div class="flex items-end">
                            <label class="flex items-center gap-2 text-sm text-slate-700">
                              <input type="checkbox" class="h-4 w-4" bind:checked={service.authEnabled} />
                              Auth required
                            </label>
                          </div>

                          <div>
                            <label class="mb-1 block text-xs font-medium text-slate-600" for={`service-${index}-provider`}>Auth provider</label>
                            <select
                              id={`service-${index}-provider`}
                              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                              bind:value={service.authProvider}
                              disabled={!service.authEnabled}
                            >
                              <option value="">Use default provider</option>
                              {#each providerNames as providerName}
                                <option value={providerName}>{providerName}</option>
                              {/each}
                            </select>
                          </div>

                          <div>
                            <label class="mb-1 block text-xs font-medium text-slate-600" for={`service-${index}-requests`}>Rate limit requests</label>
                            <input
                              id={`service-${index}-requests`}
                              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                              type="number"
                              min="1"
                              bind:value={service.rateLimitRequests}
                            />
                          </div>

                          <div>
                            <label class="mb-1 block text-xs font-medium text-slate-600" for={`service-${index}-window`}>Rate limit window</label>
                            <input
                              id={`service-${index}-window`}
                              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                              type="text"
                              placeholder="1m"
                              bind:value={service.rateLimitWindow}
                            />
                          </div>
                        </div>
                      </article>
                    {/each}
                  </div>
                {/if}
              </section>

              <div class="flex flex-wrap items-center gap-3">
                <button
                  type="button"
                  class="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-70"
                  on:click={loadConfig}
                  disabled={busy}
                >
                  Reload
                </button>
                <button
                  type="button"
                  class="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-70"
                  on:click={syncJsonFromForm}
                  disabled={busy}
                >
                  Open as JSON
                </button>
                <button
                  type="button"
                  class="rounded-lg bg-gradient-to-r from-teal-700 to-cyan-600 px-4 py-2 text-sm font-semibold text-white transition hover:from-teal-800 hover:to-cyan-700 disabled:cursor-not-allowed disabled:opacity-70"
                  on:click={saveDashboard}
                  disabled={busy}
                >
                  Save and apply
                </button>
              </div>
            </div>
          {:else}
            <div class="space-y-4">
              <p class="text-sm text-slate-500">Advanced mode for direct JSON edits and edge-case tuning.</p>

              <textarea
                class="min-h-[420px] w-full rounded-xl border border-slate-300 bg-slate-950 px-4 py-3 font-mono text-sm text-emerald-100 outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                bind:value={jsonDraft}
                spellcheck="false"
              ></textarea>

              <div class="flex flex-wrap gap-3">
                <button
                  type="button"
                  class="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-70"
                  on:click={syncFormFromJson}
                  disabled={busy}
                >
                  Sync to dashboard form
                </button>
                <button
                  type="button"
                  class="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-70"
                  on:click={loadConfig}
                  disabled={busy}
                >
                  Reload
                </button>
                <button
                  type="button"
                  class="rounded-lg bg-gradient-to-r from-teal-700 to-cyan-600 px-4 py-2 text-sm font-semibold text-white transition hover:from-teal-800 hover:to-cyan-700 disabled:cursor-not-allowed disabled:opacity-70"
                  on:click={saveJSON}
                  disabled={busy}
                >
                  Save JSON and apply
                </button>
              </div>
            </div>
          {/if}
        </div>
      {/if}

      {#if status}
        <p
          class="mt-4 text-sm"
          class:text-rose-600={statusIsError}
          class:text-slate-600={!statusIsError}
        >
          {status}
        </p>
      {/if}
    </div>
  </section>
</main>
