<script setup>
import { reactive, ref } from 'vue'
import AppShell from '../../components/AppShell.vue'
import PageHeader from '../../components/PageHeader.vue'
import FormSection from '../../components/FormSection.vue'
import { useAuthStore } from '../../stores/auth'

const auth = useAuthStore()
const loading = ref(false)
const error = ref('')
const success = ref('')

const form = reactive({
  name: auth.user?.name || '',
  email: auth.user?.email || '',
  password: ''
})

async function submit() {
  loading.value = true
  error.value = ''
  success.value = ''
  try {
    await auth.updateProfile({
      name: form.name,
      email: form.email,
      password: form.password || undefined
    })
    success.value = 'Профиль успешно обновлён'
    form.password = ''
  } catch (e) {
    error.value = e.response?.data?.message || 'Не удалось обновить профиль'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AppShell>
    <PageHeader
      title="Профиль пользователя"
      description="Управление учётной записью и персональными данными."
    />

    <div v-if="error" class="card error-text" style="margin-bottom: 20px;">{{ error }}</div>
    <div v-if="success" class="card" style="margin-bottom: 20px; color: var(--success);">{{ success }}</div>

    <form class="grid" @submit.prevent="submit">
      <FormSection title="Учётные данные" description="Изменение основных сведений и пароля.">
        <div class="form-grid">
          <div class="form-group">
            <label>Имя</label>
            <input v-model="form.name" />
          </div>
          <div class="form-group">
            <label>Email</label>
            <input v-model="form.email" type="email" />
          </div>
          <div class="form-group full">
            <label>Новый пароль</label>
            <input v-model="form.password" type="password" placeholder="Оставьте пустым, если менять не нужно" />
          </div>
        </div>
      </FormSection>

      <div class="actions">
        <button class="btn primary" :disabled="loading">
          {{ loading ? 'Сохранение...' : 'Сохранить изменения' }}
        </button>
      </div>
    </form>
  </AppShell>
</template>
