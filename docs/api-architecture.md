# Структура REST API (`api/`)

Паттерн для всего Go-модуля `api/`: слоистая архитектура `handler → service → repository`
(см. переписку/обсуждение — интерфейсы объявляет потребитель, не поставщик), плюс отдельный
WebSocket-канал для realtime-метрик. Это документ-каркас — реализация идёт по одному модулю за
раз, начиная с `controllers`.

## Дерево каталогов

```
api/
├── cmd/
│   └── api/
│       └── main.go              # composition root: config → pgxpool → repo→service→handler,
│                                 # + запуск WS-хаба/listener'а, старт chi-сервера
├── internal/
│   ├── config/
│   │   └── config.go            # типизированный конфиг (caarlos0/env): DB_DSN, JWT_SECRET, HTTP_ADDR
│   ├── model/                   # доменные типы + sentinel-ошибки — без HTTP/SQL-специфики
│   │   ├── controller.go
│   │   ├── sensor.go
│   │   ├── user.go
│   │   ├── alert_event.go
│   │   ├── sensor_reading.go
│   │   └── errors.go            # ErrNotFound, ErrDuplicate, ErrInvalidCredentials...
│   ├── repository/              # доступ к БД; реализует интерфейсы, объявленные в service
│   │   ├── sqlc/                # сгенерированный код sqlc — не редактируется руками
│   │   ├── controller_repo.go
│   │   ├── sensor_repo.go
│   │   ├── user_repo.go
│   │   ├── alert_repo.go
│   │   └── reading_repo.go      # только чтение — INSERT в sensor_readings делает Consumer (.NET)
│   ├── service/                 # бизнес-логика; каждый файл объявляет свой repo-интерфейс
│   │   ├── controller_service.go
│   │   ├── sensor_service.go
│   │   ├── auth_service.go      # регистрация/логин, выпуск JWT, bcrypt
│   │   ├── alert_service.go     # список активных алертов, resolve
│   │   └── reading_service.go
│   ├── handler/                 # HTTP-слой (chi): decode → validate → map → service → respond
│   │   ├── router.go            # сборка chi.Router, монтирование под-роутов + middleware
│   │   ├── response.go          # writeJSON/writeError — общие хелперы вместо повторения в каждой ручке
│   │   ├── controller_handler.go
│   │   ├── sensor_handler.go
│   │   ├── auth_handler.go      # POST /auth/register, /auth/login
│   │   ├── alert_handler.go
│   │   └── reading_handler.go   # GET /sensors/{id}/readings — история для графиков
│   ├── middleware/
│   │   ├── auth.go              # проверка JWT, user/role → context
│   │   ├── requirerole.go       # RBAC: admin/operator/viewer
│   │   ├── logging.go
│   │   └── cors.go
│   └── ws/                      # WebSocket — отдельно от handler: не request/response,
│       │                        # а долгоживущее соединение со своим жизненным циклом
│       ├── hub.go               # реестр подключённых клиентов + broadcast-канал
│       ├── client.go            # одно соединение: read/write pumps, ping/pong
│       ├── listener.go          # LISTEN sensor_readings (выделенный pgx-conn) → hub.Broadcast
│       └── handler.go           # GET /ws/metrics — HTTP→WS апгрейд, регистрация клиента в hub
├── db/
│   └── queries/                 # SQL-источник для sqlc (схема — в /db/migrations в корне репозитория,
│       │                        # не здесь — api/ не владеет схемой, см. docs/PROJECT.md «Миграции БД»)
│       ├── controllers.sql
│       ├── sensors.sql
│       ├── users.sql
│       ├── alert_events.sql
│       └── sensor_readings.sql
├── sqlc.yaml                    # schema: ../../db/migrations, queries: ./db/queries
├── go.mod / go.sum
├── .gitignore
└── Dockerfile                   # multi-stage: golang:1.23-alpine → distroless
```

## Почему `ws/` — отдельный пакет, а не часть `handler/`

HTTP-хендлер живёт в рамках одного запрос-ответ цикла и завершается. WebSocket-соединение
открывается один раз и живёт часами — у него свой жизненный цикл (подключение, ping/pong,
отключение), свой реестр активных клиентов и свой источник событий (не тело запроса, а `LISTEN`
из Postgres). Смешивать это с `handler/`, который построен вокруг разового запроса, только
запутает структуру. `ws/handler.go` — единственная точка соприкосновения с HTTP (сам апгрейд
`GET /ws/metrics`), всё остальное в пакете `ws` — не про HTTP.

## Как новое показание доходит до WS-клиента (решение зафиксировано)

Consumer (.NET) и API не связаны напрямую — общая точка контакта только БД (тот же принцип, что
уже действует для миграций, см. `docs/PROJECT.md`, «Миграции БД»). Механизм — **Postgres
`LISTEN`/`NOTIFY`**:

1. Триггер на `sensor_readings` (**будущая миграция `db/migrations/000002_...`, ещё не создана**)
   на каждый `INSERT` вызывает `pg_notify('sensor_readings_channel', <payload>)`.
2. `ws/listener.go` держит одно **выделенное** pgx-соединение (не из общего pool — `LISTEN`
   привязан к конкретной сессии) с `LISTEN sensor_readings_channel`.
3. При получении уведомления — передаёт его в `hub.go`, который рассылает всем подключённым
   клиентам через `client.go`.

Альтернатива (периодический polling БД) осознанно отклонена — лишняя нагрузка на БД и задержка,
привязанная к интервалу опроса, при том же по сложности решении.

## Маршруты

| Метод и путь                                    | Кто может                                 | Назначение                                                                                         |
| --------------------------------------------------------- | ------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| `POST /auth/register`                                   | публично                                  | регистрация пользователя                                                              |
| `POST /auth/login`                                      | публично                                  | логин, выдача JWT                                                                                 |
| `GET /controllers`, `GET /controllers/{id}`           | любая роль                               | чтение                                                                                                 |
| `POST /controllers`, `PATCH/DELETE /controllers/{id}` | operator, admin                                   | провижининг устройств                                                                    |
| `GET /sensors`, `GET /sensors/{id}`                   | любая роль                               | чтение                                                                                                 |
| `POST /sensors`, `PATCH/DELETE /sensors/{id}`         | operator, admin                                   | провижининг устройств                                                                    |
| `GET /sensors/{id}/readings?from=&to=`                  | любая роль                               | история показаний для графика                                                      |
| `GET /alerts?active=true`                               | любая роль                               | список алертов                                                                                  |
| `PATCH /alerts/{id}/resolve`                            | operator, admin                                   | закрыть алерт                                                                                    |
| `GET /ws/metrics`                                       | любая роль (JWT при апгрейде) | realtime-стрим новых показаний, опционально`?sensor_id=`/`?controller_id=` |

## Зависимости

Уже зафиксировано ранее в `docs/PROJECT.md`: `chi/v5` (роутер), `pgx/v5` + `pgxpool` (драйвер,
плюс `LISTEN`/`NOTIFY` для `ws/listener.go`), `sqlc` (кодогенерация, dev-time, не рантайм-зависимость
бинарника), `golang-jwt/v5` + `x/crypto/bcrypt` (auth), `go-playground/validator/v10` (валидация
DTO), `caarlos0/env/v11` (конфиг), `log/slog` (логи), `testify` (тесты).

Новое, зафиксированное этим документом: **`github.com/coder/websocket`** — WebSocket-библиотека
(бывш. `nhooyr.io/websocket`, активно поддерживается, в отличие от `gorilla/websocket`). Ранее в
`docs/PROJECT.md` был помечен как кандидат "если решится" — решено, WS реально проектируется.

## Что не в этом документе

Реализация (`.go`-файлы кроме уже существующего `cmd/api/main.go` с `/health`), миграция
`000002_...` под `NOTIFY`-триггер, сервис `api` в `docker-compose.yml` — следующие отдельные шаги,
по одному модулю за раз, начиная с `controllers`.
