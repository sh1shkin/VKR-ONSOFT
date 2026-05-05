<script setup>
defineProps({
  items: { type: Array, default: () => [] }
})
</script>

<template>
  <div class="kv-grid">
    <div v-for="item in items" :key="item.label" class="kv-item">
      <div class="kv-item__label">{{ item.label }}</div>
      <div>{{ item.value || '—' }}</div>
    </div>
  </div>
</template>
