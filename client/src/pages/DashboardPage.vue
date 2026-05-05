<script setup>
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import AppShell from '../components/AppShell.vue'
import PageHeader from '../components/PageHeader.vue'
import StatCard from '../components/StatCard.vue'
import LoadingState from '../components/LoadingState.vue'
import EmptyState from '../components/EmptyState.vue'
import BadgeStatus from '../components/BadgeStatus.vue'
import { dashboardApi } from '../api/services'

const loading = ref(true)
const summary = ref(null)
const error = ref('')

onMounted(async () => {
  try {
    const { data } = await dashboardApi.summary()
    summary.value = data
  } catch (e) {
    error.value = e.response?.data?.message || 'Не удалось загрузить сводку'
  } finally {
    loading.value = false
  }
})

function decisionVariant(label) {
  if (label === 'bid') return 'success'
  if (label === 'conditional_bid') return 'warning'
  return 'danger'
}
</script>

<template>
  <AppShell>
    <PageHeader
      title="Главная панель"
      description="Общая сводка по тендерам, компаниям и выполненным анализам."
    >
      <div class="actions">
        <RouterLink class="btn primary" to="/analysis/new">Новый анализ</RouterLink>
        <RouterLink class="btn secondary" to="/companies/new">Добавить компанию</RouterLink>
      </div>
    </PageHeader>

    <LoadingState v-if="loading" />
    <EmptyState v-else-if="error" :message="error" />

    <template v-else>
      <div class="grid three" style="margin-bottom: 20px;">
        <StatCard title="Тендеры" :value="summary?.totals?.tenders ?? 0" />
        <StatCard title="Компании" :value="summary?.totals?.companies ?? 0" />
        <StatCard title="Анализы" :value="summary?.totals?.analysisRequests ?? 0" />
      </div>

      <div class="grid two">
        <section class="card">
          <h2 class="section-title">Последние анализы</h2>
          <div v-if="!summary?.recentAnalysis?.length" class="empty-state">Анализы пока не запускались.</div>
          <div v-else class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Тендер</th>
                  <th>Компания</th>
                  <th>Решение</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in summary.recentAnalysis" :key="item.id">
                  <td>{{ item.tenderSubject }}</td>
                  <td>{{ item.companyName }}</td>
                  <td>
                    <BadgeStatus
                      :variant="decisionVariant(item.finalDecisionLabel)"
                      :text="item.finalDecisionLabel || '—'"
                    />
                  </td>
                  <td>
                    <RouterLink :to="`/analysis/results/${item.resultId}`" style="color: var(--primary); font-weight: 600;">
                      Открыть
                    </RouterLink>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="card">
          <h2 class="section-title">Последние события интеграции</h2>
          <ul v-if="summary?.integrationEvents?.length" class="list">
            <li v-for="item in summary.integrationEvents" :key="item.id">
              <div style="display:flex; justify-content: space-between; gap: 12px; align-items: center;">
                <strong>{{ item.eventType }}</strong>
                <span class="muted">{{ item.receivedAt }}</span>
              </div>
              <div class="muted" style="margin-top: 8px;">{{ item.description }}</div>
            </li>
          </ul>
          <div v-else class="empty-state">Событий пока нет.</div>
        </section>
      </div>
    </template>
  </AppShell>
</template>
