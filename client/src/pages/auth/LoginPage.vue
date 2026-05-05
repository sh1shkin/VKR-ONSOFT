<script setup>
import { reactive, ref } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import { useAuthStore } from '../../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const error = ref('')

const form = reactive({
  email: '',
  password: ''
})

async function submit() {
  error.value = ''
  try {
    await auth.login(form)
    router.push(route.query.redirect || '/')
  } catch (e) {
    error.value = e.response?.data?.message || 'Не удалось выполнить вход'
  }
}
</script>

<template>
  <div class="auth-page">
    <div class="auth-card">
      <h1 style="margin-top: 0;">Вход</h1>
      <p class="muted">Войдите в модуль генерации рекомендаций.</p>

      <form class="grid" @submit.prevent="submit">
        <div class="form-group">
          <label>Электронная почта</label>
          <input v-model="form.email" type="email" placeholder="name@example.com" required />
        </div>
        <div class="form-group">
          <label>Пароль</label>
          <input v-model="form.password" type="password" placeholder="Введите пароль" required />
        </div>
        <div v-if="error" class="error-text">{{ error }}</div>
        <button class="btn primary" :disabled="auth.loading">
          {{ auth.loading ? 'Выполняется вход...' : 'Войти' }}
        </button>
      </form>

      <p class="muted" style="margin-bottom: 0; margin-top: 20px;">
        Нет учётной записи?
        <RouterLink to="/register" style="color: var(--primary); font-weight: 700;">Зарегистрироваться</RouterLink>
      </p>
    </div>
  </div>
</template>
