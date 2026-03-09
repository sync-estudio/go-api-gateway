<script>
  export let activeTab = "dashboard";
  export let dashboardDirty = false;
  export let jsonDirty = false;
  export let onSelectTab = () => {};
</script>

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
      onSelectTab("dashboard");
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
      onSelectTab("json");
    }}
  >
    Advanced JSON {jsonDirty ? "*" : ""}
  </button>
</div>
