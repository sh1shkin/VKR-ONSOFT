<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import AppShell from '../../components/AppShell.vue'
import PageHeader from '../../components/PageHeader.vue'
import FormSection from '../../components/FormSection.vue'
import LoadingState from '../../components/LoadingState.vue'
import { companiesApi } from '../../api/services'

const props = defineProps({
  id: { type: [String, Number], default: null }
})

const router = useRouter()
const loading = ref(false)
const pageLoading = ref(Boolean(props.id))
const error = ref('')

const form = reactive({
  companyName: '',
  inn: '',
  ogrn: '',
  industry: '',
  experienceYears: 0,
  employees: 0,
  annualRevenue: 0,
  regionsOfOperation: '',
  completedContracts: '',
  hasCreditLine: false,
  availableWorkingCapital: 0,
  taxDebts: false,
  ownTransport: false,
  partnerLogistics: false,
  warehouse: false,
  certifications: '',
  knownLimitations: ''
})

const isEdit = computed(() => Boolean(props.id))

function mapResponseToForm(data) {
  form.companyName = data.companyName || ''
  form.inn = data.inn || ''
  form.ogrn = data.ogrn || ''
  form.industry = data.industry || ''
  form.experienceYears = data.experienceYears || 0
  form.employees = data.employees || 0
  form.annualRevenue = data.annualRevenue || 0
  form.regionsOfOperation = (data.regionsOfOperation || []).join('\n')
  form.completedContracts = (data.completedContracts || []).join('\n')
  form.hasCreditLine = Boolean(data.financialState?.hasCreditLine)
  form.availableWorkingCapital = data.financialState?.availableWorkingCapital || 0
  form.taxDebts = Boolean(data.financialState?.taxDebts)
  form.ownTransport = Boolean(data.logistics?.ownTransport)
  form.partnerLogistics = Boolean(data.logistics?.partnerLogistics)
  form.warehouse = Boolean(data.logistics?.warehouse)
  form.certifications = (data.certifications || []).join('\n')
  form.knownLimitations = (data.knownLimitations || []).join('\n')
}

function normalizeMultiline(value) {
  return value
    .split('\n')
    .map((item) => item.trim())
    .filter(Boolean)
}

function buildPayload() {
  return {
    companyName: form.companyName,
    inn: form.inn,
    ogrn: form.ogrn,
    industry: form.industry,
    experienceYears: Number(form.experienceYears),
    employees: Number(form.employees),
    annualRevenue: Number(form.annualRevenue),
    regionsOfOperation: normalizeMultiline(form.regionsOfOperation),
    completedContracts: normalizeMultiline(form.completedContracts),
    financialState: {
      hasCreditLine: form.hasCreditLine,
      availableWorkingCapital: Number(form.availableWorkingCapital),
      taxDebts: form.taxDebts
    },
    logistics: {
      ownTransport: form.ownTransport,
      partnerLogistics: form.partnerLogistics,
      warehouse: form.warehouse
    },
    certifications: normalizeMultiline(form.certifications),
    knownLimitations: normalizeMultiline(form.knownLimitations)
  }
}

onMounted(async () => {
  if (!props.id) return
  try {
    const { data } = await companiesApi.byId(props.id)
    mapResponseToForm(data)
  } catch (e) {
    error.value = e.response?.data?.message || 'Не удалось загрузить карточку компании'
  } finally {
    pageLoading.value = false
  }
})

async function submit() {
  loading.value = true
  error.value = ''
  try {
    const payload = buildPayload()
    if (isEdit.value) {
      await companiesApi.update(props.id, payload)
    } else {
      await companiesApi.create(payload)
    }
    router.push('/companies')
  } catch (e) {
    error.value = e.response?.data?.message || 'Не удалось сохранить карточку компании'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AppShell>
    <PageHeader
      :title="isEdit ? 'Редактирование компании' : 'Новая компания'"
      description="Заполните профиль компании, который будет использоваться при анализе участия в закупке."
    />

    <LoadingState v-if="pageLoading" />

    <template v-else>
      <div v-if="error" class="card error-text" style="margin-bottom: 20px;">{{ error }}</div>

      <form class="grid" @submit.prevent="submit">
        <FormSection title="Общие сведения" description="Базовые реквизиты и общая информация о компании.">
          <div class="form-grid">
            <div class="form-group">
              <label>Название компании</label>
              <input v-model="form.companyName" required />
            </div>
            <div class="form-group">
              <label>Отрасль</label>
              <input v-model="form.industry" />
            </div>
            <div class="form-group">
              <label>ИНН</label>
              <input v-model="form.inn" />
            </div>
            <div class="form-group">
              <label>ОГРН</label>
              <input v-model="form.ogrn" />
            </div>
            <div class="form-group">
              <label>Опыт работы, лет</label>
              <input v-model.number="form.experienceYears" type="number" min="0" />
            </div>
            <div class="form-group">
              <label>Количество сотрудников</label>
              <input v-model.number="form.employees" type="number" min="0" />
            </div>
            <div class="form-group full">
              <label>Годовая выручка</label>
              <input v-model.number="form.annualRevenue" type="number" min="0" />
            </div>
          </div>
        </FormSection>

        <FormSection title="География и опыт" description="Регионы работы и список выполненных контрактов.">
          <div class="form-grid">
            <div class="form-group full">
              <label>Регионы деятельности (по одному на строке)</label>
              <textarea v-model="form.regionsOfOperation" />
            </div>
            <div class="form-group full">
              <label>Выполненные контракты (по одному на строке)</label>
              <textarea v-model="form.completedContracts" />
            </div>
          </div>
        </FormSection>

        <FormSection title="Финансовый профиль" description="Данные для оценки финансовой устойчивости.">
          <div class="form-grid">
            <div class="form-group">
              <label>Доступный оборотный капитал</label>
              <input v-model.number="form.availableWorkingCapital" type="number" min="0" />
            </div>
            <div class="form-group">
              <label>Кредитная линия</label>
              <select v-model="form.hasCreditLine">
                <option :value="true">Есть</option>
                <option :value="false">Нет</option>
              </select>
            </div>
            <div class="form-group full">
              <label>Налоговые задолженности</label>
              <select v-model="form.taxDebts">
                <option :value="false">Нет</option>
                <option :value="true">Есть</option>
              </select>
            </div>
          </div>
        </FormSection>

        <FormSection title="Логистика и ресурсы" description="Наличие транспорта, склада и партнёрской логистики.">
          <div class="form-grid">
            <div class="form-group">
              <label>Собственный транспорт</label>
              <select v-model="form.ownTransport">
                <option :value="true">Да</option>
                <option :value="false">Нет</option>
              </select>
            </div>
            <div class="form-group">
              <label>Партнёрская логистика</label>
              <select v-model="form.partnerLogistics">
                <option :value="true">Да</option>
                <option :value="false">Нет</option>
              </select>
            </div>
            <div class="form-group full">
              <label>Склад</label>
              <select v-model="form.warehouse">
                <option :value="true">Есть</option>
                <option :value="false">Нет</option>
              </select>
            </div>
          </div>
        </FormSection>

        <FormSection title="Документы и ограничения" description="Подтверждающие документы и слабые стороны компании.">
          <div class="form-grid">
            <div class="form-group full">
              <label>Сертификаты и документы (по одному на строке)</label>
              <textarea v-model="form.certifications" />
            </div>
            <div class="form-group full">
              <label>Ограничения и слабые места (по одному на строке)</label>
              <textarea v-model="form.knownLimitations" />
            </div>
          </div>
        </FormSection>

        <div class="actions">
          <button class="btn primary" :disabled="loading">
            {{ loading ? 'Сохранение...' : isEdit ? 'Сохранить изменения' : 'Создать карточку' }}
          </button>
          <button type="button" class="btn secondary" @click="$router.push('/companies')">Отмена</button>
        </div>
      </form>
    </template>
  </AppShell>
</template>
