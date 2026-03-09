<script>
  export let enabled = false;
  export let authenticated = false;
  export let accountEmail = "";
  export let busy = false;
  export let onLogout = () => {};
</script>

<header class="border-b border-slate-200 px-6 py-5 sm:px-8">
  <div class="flex flex-wrap items-center justify-between gap-4">
    <div>
      <h1 class="text-xl font-semibold tracking-tight sm:text-2xl">Gowl - Gateway Admin Dashboard</h1>
      <p class="mt-1 text-sm text-slate-500">Manage gateway config with guided forms and live apply.</p>
    </div>

    {#if enabled && authenticated}
      <div class="flex items-center gap-3">
        <span class="rounded-full bg-teal-50 px-3 py-1 text-xs font-medium text-teal-700">{accountEmail}</span>
        <button
          type="button"
          class="rounded-lg bg-rose-600 px-3 py-2 text-sm font-semibold text-white transition hover:bg-rose-700 disabled:cursor-not-allowed disabled:opacity-70"
          on:click={onLogout}
          disabled={busy}
        >
          Logout
        </button>
      </div>
    {/if}
  </div>
</header>
