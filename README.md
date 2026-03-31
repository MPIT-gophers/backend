# mpit2026-reg backend

Backend на `Go + chi + PostgreSQL + pgx + sqlc + golang-migrate + Casbin + JWT`.

Сейчас в проекте реализованы:

- авторизация через `MAX OAuth` с локальным `JWT`
- обновление профиля пользователя (`full_name`, `phone`)
- события: создание, список моих событий, вход по invite token, получение события
- проверка доступа к событию через `Casbin`
- миграции PostgreSQL
- генерация DB-слоя через `sqlc`
- генерация Swagger/OpenAPI через комментарии в handlers

## Структура

```text
.
├── cmd/server
├── configs/model                  # casbin model + policy
├── internal
│   ├── adapters
│   │   ├── api
│   │   │   ├── action             # HTTP handlers + swagger comments
│   │   │   ├── middleware         # auth / recoverer / logger / access checks
│   │   │   └── response           # success / failure envelopes
│   │   └── logging
│   ├── config
│   ├── entities/core              # доменные DTO
│   ├── errorsStatus
│   ├── infrastructure
│   │   ├── authz                  # casbin authorizer
│   │   ├── database               # postgres, migrate, sqlc, repo impl
│   │   ├── jwt                    # issue / verify JWT
│   │   ├── max                    # клиент MAX validate API
│   │   ├── router                 # wiring repo/service/handler/routes
│   │   └── app.go                 # composition root
│   ├── repo                       # интерфейсы репозиториев
│   └── service                    # бизнес-логика
├── migrations
├── swag                           # swagger.yaml + swagger.json
├── sqlc.yaml
├── Makefile
└── docker-compose.yml
```

## Быстрый старт

```bash
cp .env.example .env
docker compose up postgres -d
make run
```

HTTP API поднимется на `http://localhost:8080`.

Полный старт в Docker:

```bash
cp .env.example .env
docker compose up --build
```

## Основные команды

```bash
make run            # локальный запуск
make build          # сборка бинарника
make test           # все тесты
make migrate-up     # применить миграции
make migrate-down   # откатить последнюю миграцию
make sqlc-generate  # перегенерировать sqlc код через docker
make swag-generate  # перегенерировать swagger из комментариев
make compose-up     # docker compose up --build
make compose-down   # docker compose down -v
```

## Конфиг

Базовый конфиг лежит в [`.env.example`](/home/gdugdh24/projects/mpit2026-reg/backend/.env.example).

Ключевые переменные:

- `HTTP_HOST`, `HTTP_PORT` — адрес HTTP-сервера
- `POSTGRES_*` — подключение к PostgreSQL
- `POSTGRES_AUTO_MIGRATE=true` — автоприменение миграций при старте
- `JWT_SECRET`, `JWT_TTL` — подпись и TTL локальных JWT
- `MAX_VALIDATE_URL`, `MAX_TIMEOUT`, `MAX_API_KEY` — интеграция с MAX validate API
- `CASBIN_MODEL_PATH`, `CASBIN_POLICY_PATH` — casbin model/policy
- `LOG_LEVEL` — уровень логирования

`.env` подхватывается автоматически через `godotenv/autoload`.

## Текущие роуты

Публичные:

- `GET /healthz`
- `POST /api/v1/auth/max/login`

Под `Authorization: Bearer <jwt>`:

- `PATCH /api/v1/me`
- `GET /api/v1/events/my`
- `POST /api/v1/events`
- `POST /api/v1/events/join-by-token`
- `GET /api/v1/events/{eventID}`

## Примеры запросов

### Login через MAX

```bash
curl --request POST \
  --url http://localhost:8080/api/v1/auth/max/login \
  --header 'Content-Type: application/json' \
  --data '{
    "token": "max-sso-token"
  }'
```

### Обновить профиль

```bash
curl --request PATCH \
  --url http://localhost:8080/api/v1/me \
  --header 'Authorization: Bearer <jwt>' \
  --header 'Content-Type: application/json' \
  --data '{
    "full_name": "Иван Иванов",
    "phone": "+79141234567"
  }'
```

Телефон в сервисе нормализуется к виду `79141234567`.

### Создать событие

```bash
curl --request POST \
  --url http://localhost:8080/api/v1/events \
  --header 'Authorization: Bearer <jwt>' \
  --header 'Content-Type: application/json' \
  --data '{
    "city": "Якутск",
    "event_date": "2026-04-15",
    "event_time": "19:30",
    "expected_guest_count": 12,
    "budget": "15000"
  }'
```

### Войти в событие по invite token

```bash
curl --request POST \
  --url http://localhost:8080/api/v1/events/join-by-token \
  --header 'Authorization: Bearer <jwt>' \
  --header 'Content-Type: application/json' \
  --data '{
    "token": "invite-token"
  }'
```

## Модель БД

Стартовая миграция: [migrations/000001_init.up.sql](/home/gdugdh24/projects/mpit2026-reg/backend/migrations/000001_init.up.sql).

Основные таблицы:

- `users` — глобальный профиль пользователя
- `user_oauth_accounts` — привязки к OAuth/SSO провайдерам (`max`, позже `telegram`)
- `events` — событие и его общий lifecycle
- `event_users` — организаторы / co-hosts
- `event_invites` — многоразовые deep-link инвайты
- `event_guests` — RSVP-гости
- `event_variants` — варианты подборки/сценария от LLM
- `event_locations` — карточки локаций внутри конкретного варианта

Ключевая связь:

- `events 1 -> N event_variants`
- `event_variants 1 -> N event_locations`
- `events.selected_variant_id` — выбранный пользователем вариант

## Миграции

Применить:

```bash
make migrate-up
```

Откатить последнюю:

```bash
make migrate-down
```

Сервер также умеет сам прогонять миграции при старте, если `POSTGRES_AUTO_MIGRATE=true`.

## sqlc

SQL-запросы лежат в:

- [internal/infrastructure/database/queries/auth.sql](/home/gdugdh24/projects/mpit2026-reg/backend/internal/infrastructure/database/queries/auth.sql)
- [internal/infrastructure/database/queries/events.sql](/home/gdugdh24/projects/mpit2026-reg/backend/internal/infrastructure/database/queries/events.sql)

Конфиг генерации: [sqlc.yaml](/home/gdugdh24/projects/mpit2026-reg/backend/sqlc.yaml).

Перегенерация:

```bash
make sqlc-generate
```

Сгенерированный код попадает в [internal/infrastructure/database/sqlc](/home/gdugdh24/projects/mpit2026-reg/backend/internal/infrastructure/database/sqlc), а repo-слой поверх него живёт в [internal/infrastructure/database](/home/gdugdh24/projects/mpit2026-reg/backend/internal/infrastructure/database).

## Swagger / OpenAPI

Swagger генерируется из комментариев в handlers и `cmd/server/main.go`.

Команда:

```bash
make swag-generate
```

Артефакты:

- [swag/swagger.yaml](/home/gdugdh24/projects/mpit2026-reg/backend/swag/swagger.yaml)
- [swag/swagger.json](/home/gdugdh24/projects/mpit2026-reg/backend/swag/swagger.json)

Swagger UI в приложение сейчас не встроен.

## Как добавлять новую фичу

Обычный поток:

1. Описать доменные DTO в [internal/entities/core](/home/gdugdh24/projects/mpit2026-reg/backend/internal/entities/core).
2. Если нужна БД, добавить или изменить миграцию в [migrations](/home/gdugdh24/projects/mpit2026-reg/backend/migrations).
3. Добавить SQL в [internal/infrastructure/database/queries](/home/gdugdh24/projects/mpit2026-reg/backend/internal/infrastructure/database/queries).
4. Выполнить `make sqlc-generate`.
5. Обновить интерфейс репозитория в [internal/repo](/home/gdugdh24/projects/mpit2026-reg/backend/internal/repo).
6. Реализовать бизнес-логику в [internal/service](/home/gdugdh24/projects/mpit2026-reg/backend/internal/service).
7. Добавить handler в [internal/adapters/api/action](/home/gdugdh24/projects/mpit2026-reg/backend/internal/adapters/api/action).
8. Подключить роут в [internal/infrastructure/router/http.go](/home/gdugdh24/projects/mpit2026-reg/backend/internal/infrastructure/router/http.go).
9. Если меняется внешний API, выполнить `make swag-generate`.

## Тесты

Сейчас в проекте есть unit-тесты на:

- `internal/service`
- `internal/adapters/api/action`
- `internal/adapters/api/middleware`
- `internal/infrastructure/jwt`

Запуск:

```bash
make test
```
