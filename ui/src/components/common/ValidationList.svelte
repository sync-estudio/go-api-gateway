<script>
  export let errors = [];
</script>

<div class="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3">
  <p class="text-sm font-semibold text-rose-700">Please fix the following:</p>
  <ul class="mt-2 list-disc space-y-1 pl-5 text-sm text-rose-700">
    {#each errors as errorMessage}
      <li>{errorMessage}</li>
    {/each}
  </ul>
</div>
