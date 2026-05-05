<script setup>
defineProps({
  title: { type: String, required: true },
  value: { type: [String, Number], required: true },
  hint: { type: String, default: '' }
})
</script>

<template>
  <div class="card stat-card">
    <p class="stat-card__title">{{ title }}</p>
    <div class="stat-card__value">{{ value }}</div>
    <p v-if="hint" class="muted" style="margin: 8px 0 0;">{{ hint }}</p>
  </div>
</template>
