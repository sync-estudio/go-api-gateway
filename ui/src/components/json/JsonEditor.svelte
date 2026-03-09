<script>
  export let jsonDraft = "";
  export let busy = false;
  export let onSyncToDashboard = () => {};
  export let onReload = () => {};
  export let onSave = () => {};
</script>

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
      on:click={onSyncToDashboard}
      disabled={busy}
    >
      Sync to dashboard form
    </button>
    <button
      type="button"
      class="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-70"
      on:click={onReload}
      disabled={busy}
    >
      Reload
    </button>
    <button
      type="button"
      class="rounded-lg bg-gradient-to-r from-teal-700 to-cyan-600 px-4 py-2 text-sm font-semibold text-white transition hover:from-teal-800 hover:to-cyan-700 disabled:cursor-not-allowed disabled:opacity-70"
      on:click={onSave}
      disabled={busy}
    >
      Save JSON and apply
    </button>
  </div>
</div>
