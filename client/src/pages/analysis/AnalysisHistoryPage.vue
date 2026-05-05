<script setup>
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import AppShell from '../../components/AppShell.vue'
import PageHeader from '../../components/PageHeader.vue'
import LoadingState from '../../components/LoadingState.vue'
import EmptyState from '../../components/EmptyState.vue'
import BadgeStatus from '../../components/BadgeStatus.vue'
import { analysisApi } from '../../api/services'

const loading = ref(true)
const error = ref('')
const items = ref([])

function variant(status) {
  if (status === 'completed') return 'success'
  if (status === 'failed') return 'danger'
  return 'warning'
}

onMounted(async () => {
  try {
    const { data } = await analysisApi.list()
    items.value = data.items || data
  } catch (e) {
    error.value = e.response?.data?.message || 'Не удалось загрузить историю анализов'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <AppShell>
    <PageHeader
      title="История анализов"
      description="Журнал всех запусков интеллектуального анализа и полученных результатов."
    />

    <LoadingState v-if="loading" />
    <EmptyState v-else-if="error" :message="error" />

    <section v-else class="card table-wrap">
      <table v-if="items.length">
        <thead>
          <tr>
            <th>ID</th>
            <th>Тендер</th>
            <th>Компания</th>
            <th>Статус</th>
            <th>Итог</th>
            <th>Дата</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in items" :key="item.id">
            <td>{{ item.id }}</td>
            <td>{{ item.tenderSubject }}</td>
            <td>{{ item.companyName }}</td>
            <td><BadgeStatus :variant="variant(item.status)" :text="item.status" /></td>
            <td>{{ item.finalDecisionLabel || '—' }}</td>
            <td>{{ item.createdAt }}</td>
            <td>
              <RouterLink v-if="item.resultId" :to="`/analysis/results/${item.resultId}`" style="color: var(--primary); font-weight: 700;">
                Открыть
              </RouterLink>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-else class="empty-state">История анализов пока пуста.</div>
    </section>
  </AppShell>
</template>
