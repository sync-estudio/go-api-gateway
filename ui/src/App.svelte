<script>
  import { onMount } from "svelte";

  import LoginForm from "./components/auth/LoginForm.svelte";
  import StatusMessage from "./components/common/StatusMessage.svelte";
  import ValidationList from "./components/common/ValidationList.svelte";
  import JsonEditor from "./components/json/JsonEditor.svelte";
  import HeaderBar from "./components/layout/HeaderBar.svelte";
  import EditorTabs from "./components/tabs/EditorTabs.svelte";
  import { METHOD_OPTIONS } from "./util/constants";
  import { requestJSON } from "./util/api";
  import {
    createDefaultForm,
    createEmptyProvider,
    createEmptyService,
  } from "./util/form";
  import { buildPayloadFromForm, toCanonical, toForm } from "./util/transform";
  import { validateForm } from "./util/validation";

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

  $: providerNames = form.providers
    .map((provider) => provider.name.trim())
    .filter(Boolean);

  $: dashboardDirty = (() => {
    try {
      return toCanonical(buildPayloadFromForm(form)) !== baselineFormCanonical;
    } catch {
      return true;
    }
  })();

  $: jsonDirty = (() => {
    try {
      return (
        toCanonical(JSON.parse(jsonDraft || "{}")) !== baselineJsonCanonical
      );
    } catch {
      return true;
    }
  })();

  function applyLoadedConfig(payload) {
    form = toForm(payload);
    jsonDraft = JSON.stringify(payload, null, 2);

    baselineFormCanonical = toCanonical(buildPayloadFromForm(form));
    baselineJsonCanonical = toCanonical(payload);
    validationErrors = [];
  }

  function setStatus(message, isError = false) {
    status = message || "";
    statusIsError = isError;
  }

  async function checkSession() {
    loadingSession = true;
    try {
      const payload = await requestJSON("/admin/api/session", {
        method: "GET",
      });

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
      defaultProvider:
        form.defaultProvider.trim() === removedName ? "" : form.defaultProvider,
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
  <section
    class="mx-auto w-full max-w-6xl rounded-2xl border border-slate-200 bg-white shadow-xl"
  >
    <HeaderBar
      {enabled}
      {authenticated}
      {accountEmail}
      {busy}
      onLogout={logout}
    />

    <div class="px-6 py-6 sm:px-8">
      {#if loadingSession}
        <div
          class="rounded-xl border border-slate-200 bg-slate-50 p-6 text-sm text-slate-600"
        >
          Checking session...
        </div>
      {:else if !enabled}
        <div
          class="rounded-xl border border-amber-300 bg-amber-50 p-6 text-sm text-amber-800"
        >
          Admin UI login is disabled. Set `ADMIN_EMAIL`, `ADMIN_PASSWORD`, and
          `ADMIN_SESSION_SECRET`.
        </div>
      {:else if !authenticated}
        <LoginForm bind:email bind:password {busy} onSubmit={login} />
      {:else}
        <div class="space-y-5">
          <EditorTabs
            {activeTab}
            {dashboardDirty}
            {jsonDirty}
            onSelectTab={(tab) => {
              activeTab = tab;
            }}
          />

          {#if validationErrors.length > 0 && activeTab === "dashboard"}
            <ValidationList errors={validationErrors} />
          {/if}

          {#if activeTab === "dashboard"}
            <div class="space-y-5">
              <section class="rounded-xl border border-slate-200 p-4">
                <h2 class="text-base font-semibold text-slate-800">Gateway</h2>
                <p class="mt-1 text-xs text-slate-500">
                  Core gateway host and port settings.
                </p>

                <div class="mt-4 grid gap-4 sm:grid-cols-2">
                  <div>
                    <label
                      class="mb-1 block text-sm font-medium text-slate-600"
                      for="proxyHost">Proxy host</label
                    >
                    <input
                      id="proxyHost"
                      class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                      type="text"
                      bind:value={form.proxyHost}
                    />
                  </div>

                  <div>
                    <label
                      class="mb-1 block text-sm font-medium text-slate-600"
                      for="proxyPort">Proxy port</label
                    >
                    <input
                      id="proxyPort"
                      class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                      type="number"
                      min="1"
                      max="65535"
                      bind:value={form.proxyPort}
                    />
                    <p class="mt-1 text-xs text-amber-700">
                      Changing port still requires process/container restart.
                    </p>
                  </div>
                </div>
              </section>

              <section class="rounded-xl border border-slate-200 p-4">
                <h2 class="text-base font-semibold text-slate-800">CORS</h2>
                <p class="mt-1 text-xs text-slate-500">
                  Browser access controls for cross-origin requests.
                </p>

                <div class="mt-4 space-y-4">
                  <label class="flex items-center gap-2 text-sm text-slate-700">
                    <input
                      type="checkbox"
                      class="h-4 w-4"
                      bind:checked={form.corsEnabled}
                    />
                    Enable CORS
                  </label>

                  <div>
                    <label
                      class="mb-1 block text-sm font-medium text-slate-600"
                      for="corsAllowedOrigins"
                      >Allowed origins (one per line)</label
                    >
                    <textarea
                      id="corsAllowedOrigins"
                      class="min-h-[88px] w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                      bind:value={form.corsAllowedOriginsText}
                    ></textarea>
                  </div>

                  <div>
                    <p class="mb-2 block text-sm font-medium text-slate-600">
                      Allowed methods
                    </p>
                    <div class="flex flex-wrap gap-3">
                      {#each METHOD_OPTIONS as method}
                        <label
                          class="flex items-center gap-2 rounded-md border border-slate-300 px-2.5 py-1.5 text-sm text-slate-700"
                        >
                          <input
                            type="checkbox"
                            value={method}
                            bind:group={form.corsAllowedMethods}
                          />
                          {method}
                        </label>
                      {/each}
                    </div>
                  </div>

                  <div class="grid gap-4 sm:grid-cols-2">
                    <div>
                      <label
                        class="mb-1 block text-sm font-medium text-slate-600"
                        for="corsAllowedHeaders"
                        >Allowed headers (one per line)</label
                      >
                      <textarea
                        id="corsAllowedHeaders"
                        class="min-h-[88px] w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                        bind:value={form.corsAllowedHeadersText}
                      ></textarea>
                    </div>

                    <div>
                      <label
                        class="mb-1 block text-sm font-medium text-slate-600"
                        for="corsExposedHeaders"
                        >Exposed headers (one per line)</label
                      >
                      <textarea
                        id="corsExposedHeaders"
                        class="min-h-[88px] w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                        bind:value={form.corsExposedHeadersText}
                      ></textarea>
                    </div>
                  </div>

                  <div class="grid gap-4 sm:grid-cols-2">
                    <label
                      class="flex items-center gap-2 text-sm text-slate-700"
                    >
                      <input
                        type="checkbox"
                        class="h-4 w-4"
                        bind:checked={form.corsAllowCredentials}
                      />
                      Allow credentials
                    </label>

                    <div>
                      <label
                        class="mb-1 block text-sm font-medium text-slate-600"
                        for="corsMaxAge">Preflight max age (seconds)</label
                      >
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
                    <h2 class="text-base font-semibold text-slate-800">
                      Auth Providers
                    </h2>
                    <p class="mt-1 text-xs text-slate-500">
                      Define available providers and select the default
                      provider.
                    </p>
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
                  <label
                    class="mb-1 block text-sm font-medium text-slate-600"
                    for="defaultProvider">Default provider</label
                  >
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
                      <article
                        class="rounded-lg border border-slate-200 bg-slate-50 p-3"
                      >
                        <div
                          class="mb-3 flex items-center justify-between gap-3"
                        >
                          <h3 class="text-sm font-semibold text-slate-700">
                            Provider #{index + 1}
                          </h3>
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
                            <label
                              class="mb-1 block text-xs font-medium text-slate-600"
                              for={`provider-${index}-name`}>Name</label
                            >
                            <input
                              id={`provider-${index}-name`}
                              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                              type="text"
                              bind:value={provider.name}
                            />
                          </div>

                          <div>
                            <label
                              class="mb-1 block text-xs font-medium text-slate-600"
                              for={`provider-${index}-type`}>Type</label
                            >
                            <select
                              id={`provider-${index}-type`}
                              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                              bind:value={provider.type}
                            >
                              <option value="jwks">jwks</option>
                            </select>
                          </div>

                          <div class="sm:col-span-2">
                            <label
                              class="mb-1 block text-xs font-medium text-slate-600"
                              for={`provider-${index}-jwks`}>JWKS URL</label
                            >
                            <input
                              id={`provider-${index}-jwks`}
                              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                              type="text"
                              bind:value={provider.jwksURL}
                            />
                          </div>

                          <div>
                            <label
                              class="mb-1 block text-xs font-medium text-slate-600"
                              for={`provider-${index}-refresh`}
                              >Refresh interval</label
                            >
                            <input
                              id={`provider-${index}-refresh`}
                              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                              type="text"
                              placeholder="1h"
                              bind:value={provider.refreshInterval}
                            />
                          </div>

                          <div>
                            <label
                              class="mb-1 block text-xs font-medium text-slate-600"
                              for={`provider-${index}-issuer`}
                              >Issuer (optional)</label
                            >
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
                    <h2 class="text-base font-semibold text-slate-800">
                      Services
                    </h2>
                    <p class="mt-1 text-xs text-slate-500">
                      Configure proxied aliases, upstream URLs, auth, and rate
                      limits.
                    </p>
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
                  <p class="mt-4 text-sm text-slate-500">
                    No services configured yet.
                  </p>
                {:else}
                  <div class="mt-4 space-y-4">
                    {#each form.services as service, index}
                      <article
                        class="rounded-lg border border-slate-200 bg-slate-50 p-3"
                      >
                        <div
                          class="mb-3 flex items-center justify-between gap-3"
                        >
                          <h3 class="text-sm font-semibold text-slate-700">
                            Service #{index + 1}
                          </h3>
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
                            <label
                              class="mb-1 block text-xs font-medium text-slate-600"
                              for={`service-${index}-url`}>Upstream URL</label
                            >
                            <input
                              id={`service-${index}-url`}
                              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                              type="text"
                              bind:value={service.url}
                            />
                          </div>

                          <div>
                            <label
                              class="mb-1 block text-xs font-medium text-slate-600"
                              for={`service-${index}-alias`}>Alias</label
                            >
                            <input
                              id={`service-${index}-alias`}
                              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                              type="text"
                              placeholder="/warehouse"
                              bind:value={service.alias}
                            />
                          </div>

                          <div class="flex items-end">
                            <label
                              class="flex items-center gap-2 text-sm text-slate-700"
                            >
                              <input
                                type="checkbox"
                                class="h-4 w-4"
                                bind:checked={service.authEnabled}
                              />
                              Auth required
                            </label>
                          </div>

                          <div>
                            <label
                              class="mb-1 block text-xs font-medium text-slate-600"
                              for={`service-${index}-provider`}
                              >Auth provider</label
                            >
                            <select
                              id={`service-${index}-provider`}
                              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                              bind:value={service.authProvider}
                              disabled={!service.authEnabled}
                            >
                              <option value="">Use default provider</option>
                              {#each providerNames as providerName}
                                <option value={providerName}
                                  >{providerName}</option
                                >
                              {/each}
                            </select>
                          </div>

                          <div>
                            <label
                              class="mb-1 block text-xs font-medium text-slate-600"
                              for={`service-${index}-requests`}
                              >Rate limit requests</label
                            >
                            <input
                              id={`service-${index}-requests`}
                              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none ring-cyan-300 transition focus:border-cyan-500 focus:ring"
                              type="number"
                              min="1"
                              bind:value={service.rateLimitRequests}
                            />
                          </div>

                          <div>
                            <label
                              class="mb-1 block text-xs font-medium text-slate-600"
                              for={`service-${index}-window`}
                              >Rate limit window</label
                            >
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
            <JsonEditor
              bind:jsonDraft
              {busy}
              onSyncToDashboard={syncFormFromJson}
              onReload={loadConfig}
              onSave={saveJSON}
            />
          {/if}
        </div>
      {/if}

      <StatusMessage {status} isError={statusIsError} />
    </div>
  </section>
</main>
