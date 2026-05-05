<script setup>
import { computed } from 'vue'

const props = defineProps({
  variant: { type: String, default: 'default' },
  text: { type: String, required: true }
})

const cssClass = computed(() => ({
  success: props.variant === 'success',
  warning: props.variant === 'warning',
  danger: props.variant === 'danger'
}))
</script>

<template>
  <span class="badge" :class="cssClass">{{ text }}</span>
</template>
