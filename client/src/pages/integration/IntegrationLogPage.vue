<script setup>
import { onMounted, ref } from 'vue'
import AppShell from '../../components/AppShell.vue'
import PageHeader from '../../components/PageHeader.vue'
import LoadingState from '../../components/LoadingState.vue'
import EmptyState from '../../components/EmptyState.vue'
import BadgeStatus from '../../components/BadgeStatus.vue'
import { integrationApi } from '../../api/services'

const loading = ref(true)
const error = ref('')
const items = ref([])

function variant(status) {
  if (status === 'processed') return 'success'
  if (status === 'failed') return 'danger'
  return 'warning'
}

onMounted(async () => {
  try {
    const { data } = await integrationApi.listMessages()
    items.value = data.items || data
  } catch (e) {
    error.value = e.response?.data?.message || 'Не удалось загрузить журнал интеграции'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <AppShell>
    <PageHeader
      title="Журнал интеграции"
      description="Сообщения, полученные из брокера, и результаты их обработки."
    />

    <LoadingState v-if="loading" />
    <EmptyState v-else-if="error" :message="error" />

    <section v-else class="card table-wrap">
      <table v-if="items.length">
        <thead>
          <tr>
            <th>ID сообщения</th>
            <th>Тип</th>
            <th>Статус</th>
            <th>Получено</th>
            <th>Ошибка</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in items" :key="item.id">
            <td>{{ item.externalMessageId || item.id }}</td>
            <td>{{ item.eventType || 'tender_received' }}</td>
            <td><BadgeStatus :variant="variant(item.processingStatus)" :text="item.processingStatus" /></td>
            <td>{{ item.receivedAt }}</td>
            <td>{{ item.errorText || '—' }}</td>
          </tr>
        </tbody>
      </table>
      <div v-else class="empty-state">Журнал интеграции пуст.</div>
    </section>
  </AppShell>
</template>
