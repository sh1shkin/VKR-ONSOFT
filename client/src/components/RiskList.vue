<script setup>
import BadgeStatus from './BadgeStatus.vue'

function variant(level) {
  if (level === 'высокий') return 'danger'
  if (level === 'средний') return 'warning'
  return 'success'
}

defineProps({
  risks: { type: Array, default: () => [] }
})
</script>

<template>
  <ul class="list">
    <li v-for="risk in risks" :key="risk.name">
      <div style="display:flex; justify-content: space-between; gap: 10px; align-items: center; margin-bottom: 10px;">
        <strong>{{ risk.name }}</strong>
        <BadgeStatus :variant="variant(risk.level)" :text="risk.level" />
      </div>
      <div class="muted" style="margin-bottom: 10px;">{{ risk.impact }}</div>
      <div v-if="risk.basis?.length">
        <div style="font-weight: 600; margin-bottom: 8px;">Основания</div>
        <ul style="margin: 0; padding-left: 18px;">
          <li v-for="item in risk.basis" :key="item">{{ item }}</li>
        </ul>
      </div>
    </li>
  </ul>
</template>
