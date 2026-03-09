<script>
  export let status = "";
  export let isError = false;
</script>

{#if status}
  <p class="mt-4 text-sm" class:text-rose-600={isError} class:text-slate-600={!isError}>
    {status}
  </p>
{/if}
