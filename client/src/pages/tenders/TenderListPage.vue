<script setup>
import { onMounted, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import AppShell from '../../components/AppShell.vue'
import PageHeader from '../../components/PageHeader.vue'
import LoadingState from '../../components/LoadingState.vue'
import EmptyState from '../../components/EmptyState.vue'
import { tendersApi } from '../../api/services'

const loading = ref(true)
const error = ref('')
const tenders = ref([])
const filters = reactive({ search: '' })

async function load() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await tendersApi.list({ search: filters.search || undefined })
    tenders.value = data.items || data
  } catch (e) {
    error.value = e.response?.data?.message || 'Не удалось загрузить тендеры'
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(() => filters.search, load)
</script>

<template>
  <AppShell>
    <PageHeader
      title="Тендеры"
      description="Закупки, полученные из модуля обработки документации через брокер сообщений."
    >
      <input v-model="filters.search" placeholder="Поиск по предмету закупки" style="min-width: 280px;" />
    </PageHeader>

    <LoadingState v-if="loading" />
    <EmptyState v-else-if="error" :message="error" />

    <section v-else class="card table-wrap">
      <table v-if="tenders.length">
        <thead>
          <tr>
            <th>ID</th>
            <th>Предмет закупки</th>
            <th>Регион</th>
            <th>НМЦК</th>
            <th>Способ закупки</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in tenders" :key="item.id">
            <td>{{ item.id }}</td>
            <td>{{ item.purchaseSubject }}</td>
            <td>{{ item.region }}</td>
            <td>{{ item.nmck }}</td>
            <td>{{ item.procurementMethod }}</td>
            <td>
              <RouterLink :to="`/tenders/${item.id}`" style="color: var(--primary); font-weight: 700;">Открыть</RouterLink>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-else class="empty-state">Тендеры пока не поступили.</div>
    </section>
  </AppShell>
</template>
