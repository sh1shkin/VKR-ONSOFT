<script setup>
import { reactive, ref } from 'vue'
import { useRouter, RouterLink } from 'vue-router'
import { useAuthStore } from '../../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const error = ref('')

const form = reactive({
  name: '',
  email: '',
  password: '',
  passwordConfirmation: ''
})

async function submit() {
  error.value = ''
  if (form.password !== form.passwordConfirmation) {
    error.value = 'Пароли не совпадают'
    return
  }

  try {
    await auth.register({
      name: form.name,
      email: form.email,
      password: form.password
    })
    router.push('/')
  } catch (e) {
    error.value = e.response?.data?.message || 'Не удалось выполнить регистрацию'
  }
}
</script>

<template>
  <div class="auth-page">
    <div class="auth-card">
      <h1 style="margin-top: 0;">Регистрация</h1>
      <p class="muted">Создайте учётную запись для работы в модуле.</p>

      <form class="grid" @submit.prevent="submit">
        <div class="form-group">
          <label>Имя</label>
          <input v-model="form.name" placeholder="Иван Иванов" required />
        </div>
        <div class="form-group">
          <label>Электронная почта</label>
          <input v-model="form.email" type="email" placeholder="name@example.com" required />
        </div>
        <div class="form-group">
          <label>Пароль</label>
          <input v-model="form.password" type="password" required />
        </div>
        <div class="form-group">
          <label>Подтверждение пароля</label>
          <input v-model="form.passwordConfirmation" type="password" required />
        </div>
        <div v-if="error" class="error-text">{{ error }}</div>
        <button class="btn primary" :disabled="auth.loading">
          {{ auth.loading ? 'Создание...' : 'Создать аккаунт' }}
        </button>
      </form>

      <p class="muted" style="margin-bottom: 0; margin-top: 20px;">
        Уже есть аккаунт?
        <RouterLink to="/login" style="color: var(--primary); font-weight: 700;">Войти</RouterLink>
      </p>
    </div>
  </div>
</template>
