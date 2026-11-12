<script setup>
defineProps({
  kind: { type: String, required: true },
})
</script>

<template>
  <span v-if="kind === 'word'" class="mark word">W</span>
  <span v-else-if="kind === 'excel'" class="mark excel">
    <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 5h14v14H5z" fill="none" stroke="#fff" stroke-width="1.6"/><path d="M5 10h14M5 15h14M10 5v14M15 5v14" stroke="#fff" stroke-width="1.2"/></svg>
  </span>
  <span v-else-if="kind === 'pdf'" class="mark pdf">P</span>
  <span v-else class="mark ppt">
    <svg viewBox="0 0 24 24" aria-hidden="true"><rect x="4" y="6" width="16" height="10" rx="1.2" fill="none" stroke="#fff" stroke-width="1.6"/><path d="M9 19h6M12 16v3" stroke="#fff" stroke-width="1.6" stroke-linecap="round"/></svg>
  </span>
</template>
