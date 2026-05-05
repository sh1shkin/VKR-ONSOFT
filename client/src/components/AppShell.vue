<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const items = [
  { label: 'Главная', to: '/' },
  { label: 'Тендеры', to: '/tenders' },
  { label: 'Компании', to: '/companies' },
  { label: 'Новый анализ', to: '/analysis/new' },
  { label: 'История анализов', to: '/analysis/history' },
  { label: 'Журнал интеграции', to: '/integration' },
  { label: 'Профиль', to: '/profile' }
]

const userName = computed(() => auth.user?.name || auth.user?.email || 'Пользователь')

function logout() {
  auth.logout()
  router.push('/login')
}
</script>

<template>
  <div class="page">
    <aside class="sidebar">
      <div>
        <div class="sidebar__title">Tender Insight</div>
        <div class="sidebar__subtitle">Модуль рекомендаций и анализа участия</div>
      </div>

      <nav>
        <RouterLink
          v-for="item in items"
          :key="item.to"
          :to="item.to"
          class="nav-link"
        >
          {{ item.label }}
        </RouterLink>
      </nav>

      <div class="card" style="margin-top: auto; background: rgba(255,255,255,0.06); border-color: rgba(255,255,255,0.1);">
        <div style="font-weight: 700; color: #fff; margin-bottom: 6px;">{{ userName }}</div>
        <div class="muted" style="color: #a8b6d1; margin-bottom: 12px;">{{ auth.user?.role || 'analyst' }}</div>
        <button class="btn secondary" @click="logout">Выйти</button>
      </div>
    </aside>

    <main class="main">
      <slot />
    </main>
  </div>
</template>
