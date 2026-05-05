<script setup>
import { computed, onMounted, ref } from 'vue'
import AppShell from '../../components/AppShell.vue'
import PageHeader from '../../components/PageHeader.vue'
import LoadingState from '../../components/LoadingState.vue'
import EmptyState from '../../components/EmptyState.vue'
import BadgeStatus from '../../components/BadgeStatus.vue'
import RiskList from '../../components/RiskList.vue'
import { analysisApi } from '../../api/services'

const props = defineProps({
  id: { type: [String, Number], required: true }
})

const loading = ref(true)
const error = ref('')
const result = ref(null)

const decisionVariant = computed(() => {
  const label = result.value?.finalDecision?.label
  if (label === 'bid') return 'success'
  if (label === 'conditional_bid') return 'warning'
  return 'danger'
})

const fitVariant = computed(() => {
  const label = result.value?.companyFit?.label
  if (label === 'fit') return 'success'
  if (label === 'conditional_fit') return 'warning'
  return 'danger'
})

onMounted(async () => {
  try {
    const { data } = await analysisApi.resultById(props.id)
    result.value = data
  } catch (e) {
    error.value = e.response?.data?.message || 'Не удалось загрузить результат анализа'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <AppShell>
    <PageHeader
      title="Результат анализа"
      description="Итог интеллектуального анализа профиля компании и условий закупки."
    />

    <LoadingState v-if="loading" />
    <EmptyState v-else-if="error" :message="error" />

    <template v-else-if="result">
      <div class="grid two" style="margin-bottom: 20px;">
        <section class="card">
          <div style="display:flex; justify-content: space-between; gap: 16px; align-items: center; margin-bottom: 12px;">
            <h2 class="section-title" style="margin:0;">Итоговое решение</h2>
            <BadgeStatus :variant="decisionVariant" :text="result.finalDecision?.label || '—'" />
          </div>
          <div style="font-size: 20px; font-weight: 700; margin-bottom: 10px;">{{ result.finalDecision?.status }}</div>
          <div class="muted">{{ result.finalDecision?.decisionComment }}</div>
          <ul class="list" style="margin-top: 16px;">
            <li v-for="(item, index) in result.finalDecision?.details || []" :key="index">{{ item }}</li>
          </ul>
        </section>

        <section class="card">
          <div style="display:flex; justify-content: space-between; gap: 16px; align-items: center; margin-bottom: 12px;">
            <h2 class="section-title" style="margin:0;">Соответствие компании</h2>
            <BadgeStatus :variant="fitVariant" :text="result.companyFit?.label || '—'" />
          </div>
          <div style="font-size: 20px; font-weight: 700; margin-bottom: 10px;">{{ result.companyFit?.status }}</div>
          <div class="muted">{{ result.companyFit?.statusComment }}</div>
          <ul class="list" style="margin-top: 16px;">
            <li v-for="(item, index) in result.companyFit?.details || []" :key="index">{{ item }}</li>
          </ul>
        </section>
      </div>

      <section class="card" style="margin-bottom: 20px;">
        <h2 class="section-title">Краткое заключение</h2>
        <p style="margin: 0; line-height: 1.7;">{{ result.summary }}</p>
      </section>

      <div class="grid two">
        <section class="card">
          <h2 class="section-title">Риски</h2>
          <RiskList :risks="result.risks || []" />
        </section>

        <section class="grid">
          <section class="card">
            <h2 class="section-title">Подводные камни</h2>
            <ul class="list">
              <li v-for="(item, index) in result.pitfalls || []" :key="index">{{ item }}</li>
            </ul>
          </section>

          <section class="card">
            <h2 class="section-title">Рекомендации</h2>
            <ul class="list">
              <li v-for="(item, index) in result.recommendations || []" :key="index">{{ item }}</li>
            </ul>
          </section>
        </section>
      </div>

      <section class="card" style="margin-top: 20px;">
        <h2 class="section-title">Оценка возможных потерь</h2>
        <div style="font-size: 18px; font-weight: 700; margin-bottom: 8px;">{{ result.lossEstimate?.status }}</div>
        <div class="muted" style="margin-bottom: 14px;">{{ result.lossEstimate?.label }}</div>
        <ul class="list">
          <li v-for="(item, index) in result.lossEstimate?.details || []" :key="index">{{ item }}</li>
        </ul>
      </section>

      <section class="card" style="margin-top: 20px;">
        <h2 class="section-title">Raw JSON</h2>
        <pre class="pre">{{ JSON.stringify(result, null, 2) }}</pre>
      </section>
    </template>
  </AppShell>
</template>
