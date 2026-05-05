<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppShell from '../../components/AppShell.vue'
import PageHeader from '../../components/PageHeader.vue'
import FormSection from '../../components/FormSection.vue'
import LoadingState from '../../components/LoadingState.vue'
import KeyValueGrid from '../../components/KeyValueGrid.vue'
import { analysisApi, companiesApi, tendersApi } from '../../api/services'

const route = useRoute()
const router = useRouter()
const loading = ref(true)
const submitting = ref(false)
const error = ref('')
const tenders = ref([])
const companies = ref([])
const selectedTender = ref(null)
const selectedCompany = ref(null)

const form = reactive({
  tenderId: route.query.tenderId || '',
  companyId: route.query.companyId || ''
})

onMounted(async () => {
  try {
    const [{ data: tendersData }, { data: companiesData }] = await Promise.all([
      tendersApi.list(),
      companiesApi.list()
    ])
    tenders.value = tendersData.items || tendersData
    companies.value = companiesData.items || companiesData
    updateSelections()
  } catch (e) {
    error.value = e.response?.data?.message || 'Не удалось загрузить данные для подготовки анализа'
  } finally {
    loading.value = false
  }
})

function updateSelections() {
  selectedTender.value = tenders.value.find((item) => String(item.id) === String(form.tenderId)) || null
  selectedCompany.value = companies.value.find((item) => String(item.id) === String(form.companyId)) || null
}

const tenderMeta = computed(() => {
  if (!selectedTender.value) return []
  return [
    { label: 'Предмет закупки', value: selectedTender.value.purchaseSubject },
    { label: 'Регион', value: selectedTender.value.region },
    { label: 'НМЦК', value: selectedTender.value.nmck },
    { label: 'Способ закупки', value: selectedTender.value.procurementMethod }
  ]
})

const companyMeta = computed(() => {
  if (!selectedCompany.value) return []
  return [
    { label: 'Компания', value: selectedCompany.value.companyName },
    { label: 'Отрасль', value: selectedCompany.value.industry },
    { label: 'Опыт', value: `${selectedCompany.value.experienceYears || 0} лет` },
    { label: 'Выручка', value: selectedCompany.value.annualRevenue }
  ]
})

async function submit() {
  if (!form.tenderId || !form.companyId) {
    error.value = 'Выберите тендер и компанию'
    return
  }
  submitting.value = true
  error.value = ''
  try {
    const { data } = await analysisApi.create({
      tenderId: Number(form.tenderId),
      companyId: Number(form.companyId)
    })
    router.push(`/analysis/results/${data.resultId || data.id}`)
  } catch (e) {
    error.value = e.response?.data?.message || 'Не удалось запустить анализ'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <AppShell>
    <PageHeader
      title="Подготовка анализа"
      description="Выберите тендер и компанию. После этого система сформирует рекомендации на основе LLM."
    />

    <LoadingState v-if="loading" />

    <template v-else>
      <div v-if="error" class="card error-text" style="margin-bottom: 20px;">{{ error }}</div>

      <form class="grid" @submit.prevent="submit">
        <FormSection title="Выбор данных для анализа" description="Система объединит данные тендера и профиль компании.">
          <div class="form-grid">
            <div class="form-group">
              <label>Тендер</label>
              <select v-model="form.tenderId" @change="updateSelections">
                <option value="">Выберите тендер</option>
                <option v-for="item in tenders" :key="item.id" :value="item.id">
                  {{ item.purchaseSubject }}
                </option>
              </select>
            </div>
            <div class="form-group">
              <label>Компания</label>
              <select v-model="form.companyId" @change="updateSelections">
                <option value="">Выберите компанию</option>
                <option v-for="item in companies" :key="item.id" :value="item.id">
                  {{ item.companyName }}
                </option>
              </select>
            </div>
          </div>
        </FormSection>

        <div class="split">
          <FormSection title="Выбранный тендер" description="Основные сведения по закупке.">
            <KeyValueGrid :items="tenderMeta" />
            <div v-if="selectedTender?.keyRequirements?.length" style="margin-top: 18px;">
              <h3 class="section-title" style="font-size: 16px;">Ключевые требования</h3>
              <ul class="list">
                <li v-for="(item, index) in selectedTender.keyRequirements" :key="index">{{ item }}</li>
              </ul>
            </div>
          </FormSection>

          <FormSection title="Выбранная компания" description="Основные параметры профиля компании.">
            <KeyValueGrid :items="companyMeta" />
            <div v-if="selectedCompany?.knownLimitations?.length" style="margin-top: 18px;">
              <h3 class="section-title" style="font-size: 16px;">Ограничения</h3>
              <ul class="list">
                <li v-for="(item, index) in selectedCompany.knownLimitations" :key="index">{{ item }}</li>
              </ul>
            </div>
          </FormSection>
        </div>

        <div class="actions">
          <button class="btn primary" :disabled="submitting">
            {{ submitting ? 'Выполняется анализ...' : 'Сформировать рекомендации' }}
          </button>
        </div>
      </form>
    </template>
  </AppShell>
</template>
