Что положить в проект:

1. Папку llm -> в корень проекта, рядом с server, client, datasets.
2. Файл docker-compose.llm.yaml -> в корень проекта.
3. Файл docker-compose.with-llm.yaml -> в корень проекта, если хочешь заменить основной compose целиком.
4. Файл .env.llm.example -> в корень проекта как пример.

Рекомендуемый способ:
- оставить твой текущий docker-compose.yaml как есть;
- добавить llm/ и docker-compose.llm.yaml;
- запускать так:

  docker compose -f docker-compose.yaml -f docker-compose.llm.yaml --profile llm up --build -d

Проверка:
- LLM health: http://localhost:8000/health
- Backend health: http://localhost:8080/api/health
- Frontend: http://localhost:5173

Важно:
- сервис llm рассчитан на запуск с GPU; на CPU Qwen2.5-7B + LoRA будет очень медленным или может не запуститься;
- если у тебя нет локальной GPU-поддержки в Docker, подними llm-сервис на сервере с GPU и в compose/переменных укажи
  LLM_SERVICE_URL=http://<адрес-сервера>:8000
- Go backend шлёт в llm camelCase-поля, а модель обучалась на snake_case. Сервис автоматически нормализует input к snake_case,
  а output обратно приводит к camelCase, чтобы серверу ничего не ломать.
