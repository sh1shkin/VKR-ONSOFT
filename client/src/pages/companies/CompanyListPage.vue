<script setup>
import { onMounted, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import AppShell from '../../components/AppShell.vue'
import PageHeader from '../../components/PageHeader.vue'
import LoadingState from '../../components/LoadingState.vue'
import EmptyState from '../../components/EmptyState.vue'
import { companiesApi } from '../../api/services'

const companies = ref([])
const loading = ref(true)
const error = ref('')
const filters = reactive({ search: '' })

async function load() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await companiesApi.list({ search: filters.search || undefined })
    companies.value = data.items || data
  } catch (e) {
    error.value = e.response?.data?.message || 'Не удалось загрузить компании'
  } finally {
    loading.value = false
  }
}

async function removeCompany(id) {
  if (!window.confirm('Удалить карточку компании?')) return
  try {
    await companiesApi.remove(id)
    await load()
  } catch (e) {
    alert(e.response?.data?.message || 'Не удалось удалить компанию')
  }
}

onMounted(load)
watch(() => filters.search, load)
</script>

<template>
  <AppShell>
    <PageHeader
      title="Компании"
      description="Реестр профилей компаний, используемых для генерации рекомендаций."
    >
      <div class="actions">
        <input v-model="filters.search" placeholder="Поиск по названию компании" />
        <RouterLink class="btn primary" to="/companies/new">Добавить компанию</RouterLink>
      </div>
    </PageHeader>

    <LoadingState v-if="loading" />
    <EmptyState v-else-if="error" :message="error" />

    <section v-else class="card table-wrap">
      <table v-if="companies.length">
        <thead>
          <tr>
            <th>Название</th>
            <th>Отрасль</th>
            <th>Стаж</th>
            <th>Выручка</th>
            <th>Сотрудники</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in companies" :key="item.id">
            <td>{{ item.companyName }}</td>
            <td>{{ item.industry }}</td>
            <td>{{ item.experienceYears }} лет</td>
            <td>{{ item.annualRevenue }}</td>
            <td>{{ item.employees }}</td>
            <td>
              <div class="actions">
                <RouterLink :to="`/companies/${item.id}/edit`" style="color: var(--primary); font-weight: 700;">Изменить</RouterLink>
                <button class="btn danger" @click="removeCompany(item.id)">Удалить</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-else class="empty-state">Компании ещё не добавлены.</div>
    </section>
  </AppShell>
</template>
