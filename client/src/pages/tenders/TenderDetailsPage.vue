<script setup>
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import AppShell from '../../components/AppShell.vue'
import PageHeader from '../../components/PageHeader.vue'
import LoadingState from '../../components/LoadingState.vue'
import EmptyState from '../../components/EmptyState.vue'
import KeyValueGrid from '../../components/KeyValueGrid.vue'
import { tendersApi } from '../../api/services'

const props = defineProps({
  id: { type: [String, Number], required: true }
})

const tender = ref(null)
const loading = ref(true)
const error = ref('')

const metaItems = computed(() => {
  if (!tender.value) return []
  return [
    { label: 'Регион', value: tender.value.region },
    { label: 'НМЦК', value: tender.value.nmck },
    { label: 'Обеспечение заявки', value: tender.value.securityBid },
    { label: 'Обеспечение контракта', value: tender.value.securityContract },
    { label: 'Метод закупки', value: tender.value.procurementMethod },
    { label: 'Место поставки', value: tender.value.deliveryPlace },
    { label: 'Срок поставки', value: tender.value.deliveryPeriod },
    { label: 'Условия оплаты', value: tender.value.paymentTerms }
  ]
})

onMounted(async () => {
  try {
    const { data } = await tendersApi.byId(props.id)
    tender.value = data
  } catch (e) {
    error.value = e.response?.data?.message || 'Не удалось загрузить тендер'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <AppShell>
    <PageHeader
      title="Карточка тендера"
      description="Просмотр полной информации по закупке, полученной из первого модуля."
    >
      <div class="actions">
        <RouterLink class="btn secondary" to="/tenders">Назад к списку</RouterLink>
        <RouterLink class="btn primary" :to="`/analysis/new?tenderId=${id}`">Анализировать</RouterLink>
      </div>
    </PageHeader>

    <LoadingState v-if="loading" />
    <EmptyState v-else-if="error" :message="error" />

    <template v-else-if="tender">
      <section class="card" style="margin-bottom: 20px;">
        <h2 class="section-title">{{ tender.purchaseSubject }}</h2>
        <KeyValueGrid :items="metaItems" />
      </section>

      <div class="grid two">
        <section class="card">
          <h2 class="section-title">Краткое описание</h2>
          <ul class="list">
            <li v-for="(item, index) in tender.tenderSummary || []" :key="index">{{ item }}</li>
          </ul>
        </section>

        <section class="card">
          <h2 class="section-title">Ключевые требования</h2>
          <ul class="list">
            <li v-for="(item, index) in tender.keyRequirements || []" :key="index">{{ item }}</li>
          </ul>
        </section>
      </div>

      <section class="card" style="margin-top: 20px;">
        <h2 class="section-title">Полный перечень требований</h2>
        <ul class="list">
          <li v-for="(item, index) in tender.allRequirements || []" :key="index">{{ item }}</li>
        </ul>
      </section>
    </template>
  </AppShell>
</template>
