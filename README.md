# mpit2026-reg backend

Стартовый backend-каркас на `Go + net/http + chi + PostgreSQL + sqlc + pgx + golang-migrate + env + slog`.

Проект уже содержит:

- HTTP API на `chi`
- конфиг через `.env`
- подключение к PostgreSQL через `pgxpool`
- миграции через `golang-migrate`
- слой `repo` + `usecase`
- стартовый домен `registrations`
- `sqlc`-конфиг и SQL-запросы
- Docker и docker-compose для локального старта

## Структура

```text
.
├── cmd/server                  # вход в приложение и CLI-команды
├── configs/model              # описание модели конфигурации
├── infra
├── internal
│   ├── adapters
│   │   ├── api
│   │   │   ├── action         # HTTP handlers
│   │   │   ├── middleware     # HTTP middleware
│   │   │   └── response       # единый формат JSON-ответов
│   │   └── logging            # инициализация slog
│   ├── config                 # загрузка env-конфига
│   ├── entities/core          # доменные сущности
│   ├── errorsStatus           # ошибки домена и HTTP-маппинг
│   ├── infrastructure
│   │   ├── database           # pgx, migrate, sqlc, repo impl
│   │   ├── router             # сборка chi router
│   │   └── app.go             # композиция приложения
│   ├── repo                   # интерфейсы репозиториев
│   ├── usecase                # бизнес-логика
│   └── utils
├── migrations                 # SQL migrations
├── pkg
├── swag                       # минимальная OpenAPI-спека
└── tools
```

## Быстрый старт локально

1. Скопировать env:

```bash
cp .env.example .env
```

2. Поднять PostgreSQL:

```bash
docker compose up postgres -d
```

3. Запустить приложение:

```bash
make run
```

Приложение поднимется на `http://localhost:8080`.

## Быстрый старт в Docker

```bash
cp .env.example .env
docker compose up --build
```

Сервисы:

- API: `http://localhost:8080`
- PostgreSQL: `localhost:5432`

## Основные команды

```bash
make run            # локальный запуск
make build          # сборка бинарника
make test           # запуск тестов
make migrate-up     # применить миграции
make migrate-down   # откатить последнюю миграцию
make sqlc-generate  # перегенерировать код sqlc через docker
make compose-up     # поднять проект в docker compose
make compose-down   # остановить docker compose
```

## .env.example

В репозитории уже есть файл [.env.example](/home/gdugdh24/projects/mpit2026-reg/backend/.env.example) с минимальным набором переменных.

Ключевые переменные:

- `HTTP_HOST`, `HTTP_PORT` определяют адрес HTTP-сервера
- `POSTGRES_*` определяют параметры подключения к БД
- `POSTGRES_AUTO_MIGRATE=true` включает автоприменение миграций при старте сервера
- `POSTGRES_MIGRATIONS_PATH=file://migrations` указывает путь для `golang-migrate`
- `LOG_LEVEL` задает уровень логирования (`debug`, `info`, `warn`, `error`)

`.env` подхватывается автоматически при локальном запуске через `godotenv/autoload`, поэтому достаточно создать файл рядом с `go.mod`.

## Логи

Логи пишутся через `slog` в JSON и используют единый набор ключей.

Базовые поля:

- `time`
- `level`
- `msg`
- `service`
- `env`

Для HTTP-запросов дополнительно пишутся:

- `request_id`
- `trace_id`
- `correlation_id`
- `http_method`
- `http_path`
- `http_route`
- `http_host`
- `remote_ip`
- `user_agent`
- `http_status`
- `response_bytes`
- `duration_ms`

Поддерживаемые входящие заголовки трассировки:

- `X-Request-ID`
- `X-Correlation-ID`
- `X-Trace-ID`
- `Traceparent`

Если они не переданы, сервис сам генерирует идентификаторы и возвращает их обратно в response headers.

## API

### Healthcheck

```bash
curl --request GET \
  --url http://localhost:8080/healthz
```

Пример ответа:

```json
{
  "data": {
    "status": "ok",
    "database": "ok"
  }
}
```

### Создать регистрацию

```bash
curl --request POST \
  --url http://localhost:8080/api/v1/registrations \
  --header 'Content-Type: application/json' \
  --data '{
    "full_name": "Ivan Ivanov",
    "email": "ivan@example.com"
  }'
```

Пример ответа:

```json
{
  "data": {
    "id": 1,
    "full_name": "Ivan Ivanov",
    "email": "ivan@example.com",
    "created_at": "2026-03-31T12:00:00Z"
  }
}
```

### Получить список регистраций

```bash
curl --request GET \
  --url http://localhost:8080/api/v1/registrations
```

Пример ответа:

```json
{
  "data": [
    {
      "id": 1,
      "full_name": "Ivan Ivanov",
      "email": "ivan@example.com",
      "created_at": "2026-03-31T12:00:00Z"
    }
  ]
}
```

## Как добавлять роуты

Базовый поток такой:

1. Описать доменную модель или входные данные в [internal/entities/core](/home/gdugdh24/projects/mpit2026-reg/backend/internal/entities/core).
2. Если нужен доступ к БД, добавить SQL в [internal/infrastructure/database/queries](/home/gdugdh24/projects/mpit2026-reg/backend/internal/infrastructure/database/queries) и обновить код `sqlc`.
3. Расширить интерфейс репозитория в [internal/repo](/home/gdugdh24/projects/mpit2026-reg/backend/internal/repo).
4. Реализовать бизнес-логику в [internal/usecase](/home/gdugdh24/projects/mpit2026-reg/backend/internal/usecase).
5. Создать HTTP handler в [internal/adapters/api/action](/home/gdugdh24/projects/mpit2026-reg/backend/internal/adapters/api/action).
6. Подключить новый endpoint в [internal/infrastructure/router/http.go](/home/gdugdh24/projects/mpit2026-reg/backend/internal/infrastructure/router/http.go).
7. Если это внешний API, обновить [swag/openapi.yaml](/home/gdugdh24/projects/mpit2026-reg/backend/swag/openapi.yaml).

Минимальный пример регистрации нового роута:

```go
func NewRouter(logger *slog.Logger, health *action.HealthHandler, registrations *action.RegistrationHandler) http.Handler {
    r := chi.NewRouter()

    r.Get("/healthz", health.Get)

    r.Route("/api/v1", func(r chi.Router) {
        r.Get("/registrations", registrations.List)
        r.Post("/registrations", registrations.Create)
        r.Get("/new-resource", someHandler.List)
    })

    return r
}
```

Если роут требует новую таблицу или поля:

1. Добавить миграцию в [migrations](/home/gdugdh24/projects/mpit2026-reg/backend/migrations)
2. Добавить SQL-запросы для `sqlc`
3. Перегенерировать код:

```bash
make sqlc-generate
```

## Миграции

Применить миграции:

```bash
make migrate-up
```

Откатить последнюю миграцию:

```bash
make migrate-down
```

Также сервер умеет выполнять миграции сам при старте, если `POSTGRES_AUTO_MIGRATE=true`.

## sqlc

SQL-запросы лежат в [internal/infrastructure/database/queries/registrations.sql](/home/gdugdh24/projects/mpit2026-reg/backend/internal/infrastructure/database/queries/registrations.sql), конфиг `sqlc` в [sqlc.yaml](/home/gdugdh24/projects/mpit2026-reg/backend/sqlc.yaml).

Перегенерация:

```bash
make sqlc-generate
```

## OpenAPI

Минимальная спецификация лежит в [swag/openapi.yaml](/home/gdugdh24/projects/mpit2026-reg/backend/swag/openapi.yaml).
