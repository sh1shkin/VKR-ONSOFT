<script setup>
defineProps({
  title: { type: String, required: true },
  description: { type: String, default: '' }
})
</script>

<template>
  <section class="card">
    <h2 class="section-title">{{ title }}</h2>
    <p v-if="description" class="muted" style="margin-top: -4px; margin-bottom: 18px;">{{ description }}</p>
    <slot />
  </section>
</template>
