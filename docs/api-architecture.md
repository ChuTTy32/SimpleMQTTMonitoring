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
│   ├── ws/                      # WebSocket — отдельно от handler: не request/response,
│   │   │                        # а долгоживущее соединение со своим жизненным циклом
│   │   ├── hub.go               # реестр подключённых клиентов + broadcast-канал
│   │   ├── client.go            # одно соединение: read/write pumps, ping/pong
│   │   ├── listener.go          # LISTEN sensor_readings (выделенный pgx-conn) → hub.Broadcast
│   │   └── handler.go           # GET /ws/metrics — HTTP→WS апгрейд, регистрация клиента в hub
│   ├── analytics/                # LLM Analytics — свой pgxpool (роль analytics_readonly),
│   │   │                         # не переиспользует repository/ основного API, см. ниже
│   │   ├── handler.go            # POST /analytics/query — decode → agent.Run → respond
│   │   ├── agent.go              # tool-calling цикл: LLM ⇄ tools, maxIterations, логирование
│   │   ├── tools.go              # dispatch-таблица {"get_controller_sensors": ..., "get_sensor_summary": ...}
│   │   └── repo.go               # SQL под read-only ролью: controllers/sensors/sensor_readings_hourly
│   └── llm/
│       └── client.go             # Client interface: Generate(messages, tools) — провайдер
│                                  # (Claude/OpenAI/...) спрятан за интерфейсом, agent.go его не знает
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

1. Триггер на `sensor_readings` (**будущая миграция `db/migrations/000003_...`, ещё не создана** —
   `000002` занята LLM Analytics, см. ниже) на каждый `INSERT` вызывает
   `pg_notify('sensor_readings_channel', <payload>)`.
2. `ws/listener.go` держит одно **выделенное** pgx-соединение (не из общего pool — `LISTEN`
   привязан к конкретной сессии) с `LISTEN sensor_readings_channel`.
3. При получении уведомления — передаёт его в `hub.go`, который рассылает всем подключённым
   клиентам через `client.go`.

Альтернатива (периодический polling БД) осознанно отклонена — лишняя нагрузка на БД и задержка,
привязанная к интервалу опроса, при том же по сложности решении.

## LLM Analytics (`internal/analytics`, `internal/llm`) — решение зафиксировано

Задача: пользователь задаёт вопрос на естественном языке («как вела себя температура датчика X
за неделю», «сравни контроллеры 1 и 2») и получает связный текстовый ответ. Полная концепция
(включая отклонённые на этом этапе варианты — отдельный микросервис, Python-sandbox для
произвольного агентного анализа) обсуждена и сведена к минимально достаточной архитектуре:
**LLM — не более чем парсер естественного языка в аргументы функций** (tool calling), вся
логика получения и агрегации данных остаётся детерминированным Go+SQL кодом. Из исходной
концепции сознательно исключено на этом этапе:

- **отдельный микросервис `llm-analytics/`** — не оправдан на один эндпоинт; это модуль
  `internal/analytics` внутри уже существующего `api/`. Выносить в отдельный сервис — только
  когда появится вторая реальная причина (например, независимое масштабирование), не «на
  будущее»;
- **Python-sandbox и произвольная генерация кода** — отдельный security-проект сам по себе
  (изоляция контейнера, лимиты CPU/RAM, запрет Docker socket и т.д.), непропорционально
  рискованный относительно текущей задачи. Если появится реальный сценарий, для которого
  недостаточно конечного набора tool'ов (например, поиск неожиданных корреляций между
  метриками) — это отдельное архитектурное решение, не молчаливое расширение текущего кода;
- **отдельная таблица `llm_context` с версионированием** — контекст (какие есть контроллеры,
  датчики, единицы измерения) вычисляется на лету простым `SELECT` из `controllers`/`sensors`
  при каждом запросе, а не материализуется и не синхронизируется отдельным Context Builder'ом.
  Датасет достаточно маленький, чтобы это было дешевле, чем поддерживать вторую копию данных.

### Tool-calling цикл

`agent.go` реализует классический для LLM-провайдеров (Claude/OpenAI и т.д.) паттерн function
calling — не самописный agent-framework:

```
messages := [userQuery]
for i := 0; i < maxIterations; i++ {
    resp := llm.Generate(messages, tools)
    if resp.StopReason != "tool_use" {
        return resp.Text   // финальный связный ответ — отдаём пользователю
    }
    for _, call := range resp.ToolCalls {        // может быть несколько за один ответ
        result := dispatch(call.Name, call.Args) // tools.go — таблица {имя: SQL-функция}
        messages = append(messages, toolResult(call.ID, result))
    }
}
return errBudgetExceeded
```

Для сложного вопроса («сравни контроллеры 1 и 2 за неделю, где хуже») LLM сама решает вызвать
`get_controller_sensors` дважды и `get_sensor_summary` для каждого найденного датчика, после чего
сравнивает агрегаты и формулирует ответ — это происходит в контексте самой LLM между итерациями
цикла, backend не поддерживает сравнение специальным кодом.

**Обязательные ограничители (не опциональны для MVP):**

- `maxIterations` (например, 5-6) — без него LLM потенциально может зациклиться на вызовах
  инструментов, а стоимость растёт с каждым round-trip к провайдеру;
- `slog` на каждый tool call (имя, аргументы, номер итерации) — дёшево сделать сразу, дорого
  добавлять постфактум при разборе инцидента.

### Инструменты (tools.go)

Только под реально нужный сценарий (сводка/сравнение), не заранее под гипотетические
формулировки:

| Tool | Параметры | SQL-источник |
|---|---|---|
| `get_controller_sensors` | `controller_id` | `sensors` (по `controller_id`) |
| `get_sensor_summary` | `sensor_id`, `from`, `to` | `sensor_readings_hourly` (continuous aggregate — не сырые `sensor_readings`, см. `db/migrations/000002_llm_analytics_readonly.up.sql`) |

Оба tool'а — обычные Go-функции с параметризованным SQL, никакого динамически формируемого
запроса. LLM не может расширить, изменить или обойти этот SQL — она только заполняет параметры.

### Доступ к БД — read-only роль

`internal/analytics/repo.go` подключается к Postgres **отдельным** `pgxpool` под ролью
`analytics_readonly` (не под тем же пулом/пользователем, что основной `repository/`) —
физически не имеет `INSERT`/`UPDATE`/`DELETE`, даже если в коде появится баг. Роль и `GRANT`
созданы в `db/migrations/000002_llm_analytics_readonly.up.sql`: доступ дан только на
`controllers`, `sensors`, `sensor_readings`, `sensor_readings_hourly` — `users` (хеши паролей),
`alert_events`, `system_settings` осознанно исключены, пока под них нет tool'а. Та же миграция
добавляет `stddev_value`/`reading_count` в `sensor_readings_hourly` (требует пересоздания
continuous aggregate — `ALTER` для него недоступен в TimescaleDB).

### LLM-провайдер спрятан за интерфейсом

```go
// internal/llm/client.go
type Client interface {
    Generate(ctx context.Context, messages []Message, tools []ToolDef) (Response, error)
}
```

`agent.go` работает только с этим интерфейсом, не с конкретным SDK — замена провайдера
(например, Claude → OpenAI) не требует правок в `analytics/`. Конкретная реализация — прямые
HTTP-вызовы к REST API выбранного провайдера (без тяжёлого официального SDK ради одного
интерфейса); ключ — `LLM_API_KEY` в `.env` (см. `.env.example`).

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
| `POST /analytics/query`                                 | любая роль                               | LLM-сводка/анализ по естественному языку — см. «LLM Analytics» выше |

## Зависимости

Уже зафиксировано ранее в `docs/PROJECT.md`: `chi/v5` (роутер), `pgx/v5` + `pgxpool` (драйвер,
плюс `LISTEN`/`NOTIFY` для `ws/listener.go`), `sqlc` (кодогенерация, dev-time, не рантайм-зависимость
бинарника), `golang-jwt/v5` + `x/crypto/bcrypt` (auth), `go-playground/validator/v10` (валидация
DTO), `caarlos0/env/v11` (конфиг), `log/slog` (логи), `testify` (тесты).

Новое, зафиксированное этим документом: **`github.com/coder/websocket`** — WebSocket-библиотека
(бывш. `nhooyr.io/websocket`, активно поддерживается, в отличие от `gorilla/websocket`). Ранее в
`docs/PROJECT.md` был помечен как кандидат "если решится" — решено, WS реально проектируется.

LLM Analytics не добавляет тяжёлых зависимостей — `internal/llm/client.go` реализован через
`net/http` к REST API выбранного провайдера, без официального SDK (см. «LLM-провайдер спрятан за
интерфейсом» выше).

## Что не в этом документе

Реализация (`.go`-файлы кроме уже существующего `cmd/api/main.go` с `/health`), миграция
`000003_...` под `NOTIFY`-триггер (`000002` теперь занята LLM Analytics, см. выше), сервис `api`
в `docker-compose.yml` — следующие отдельные шаги, по одному модулю за раз, начиная с
`controllers`.
