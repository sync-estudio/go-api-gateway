<script>
  export let email = "";
  export let password = "";
  export let busy = false;
  export let onSubmit = () => {};
</script>

<form class="mx-auto max-w-md space-y-4" on:submit={onSubmit}>
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
