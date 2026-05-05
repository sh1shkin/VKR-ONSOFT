<script setup>
defineProps({
  message: { type: String, default: 'Данные отсутствуют' }
})
</script>

<template>
  <div class="card empty-state">{{ message }}</div>
</template>
