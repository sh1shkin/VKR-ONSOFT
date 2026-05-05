# Client

Клиентская часть модуля интеллектуальной поддержки принятия решения по участию компании в закупке.

## Стек
- Vue 3
- Vue Router
- Pinia
- Axios
- Vite

## Запуск
```bash
npm install
cp .env.example .env
npm run dev
```

## Ожидаемые backend endpoints

### Auth
- POST `/auth/login`
- POST `/auth/register`
- GET `/auth/me`
- PUT `/auth/me`

### Dashboard
- GET `/dashboard/summary`

### Companies
- GET `/companies`
- GET `/companies/:id`
- POST `/companies`
- PUT `/companies/:id`
- DELETE `/companies/:id`

### Tenders
- GET `/tenders`
- GET `/tenders/:id`

### Analysis
- POST `/analysis/requests`
- GET `/analysis/requests`
- GET `/analysis/requests/:id`
- GET `/analysis/results/:id`

### Integration
- GET `/integration/messages`

## Что уже реализовано
- авторизация и регистрация;
- дашборд;
- список тендеров и карточка тендера;
- список компаний;
- создание и редактирование карточки компании;
- подготовка анализа;
- просмотр результата анализа;
- история анализов;
- журнал интеграции;
- профиль пользователя.
