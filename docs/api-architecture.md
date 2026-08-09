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
├── sqlc.yaml                    # schema: ../db/migrations, queries: ./db/queries
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

## WebSocket: два потока событий, один канал (решение зафиксировано)

Consumer (.NET) и API не связаны напрямую — общая точка контакта только БД (тот же принцип, что
уже действует для миграций, см. `docs/PROJECT.md`, «Миграции БД»). Механизм — **Postgres
`LISTEN`/`NOTIFY`**. Realtime-событий в системе два вида — новое показание и срабатывание/закрытие
алерта — но WS-эндпоинт, `hub` и `client` **общие**: connection-management (регистрация клиента,
ping/pong, рассылка байт подписчикам) не зависит от смысла payload, дублировать его под второй
физический эндпоинт означало бы копировать код ради копирования, а фронту — держать два сокета
вместо одного (двойной хендшейк, двойной реконнект, двойная JWT-проверка при апгрейде). Разделены
только источники событий — два независимых listener'а на два разных `NOTIFY`-канала, оба пишут в
общий `hub.Broadcast`:

1. **`listener_readings.go`** — `LISTEN sensor_readings_channel`. Триггер на `sensor_readings`
   (**будущая миграция `db/migrations/000003_...`, ещё не создана** — `000002` занята LLM
   Analytics) на каждый `INSERT` вызывает `pg_notify('sensor_readings_channel', <payload>)`.
2. **`listener_alerts.go`** — `LISTEN alert_events_channel`. Триггер на `alert_events`
   (**будущая миграция `db/migrations/000004_...`**) на `AFTER INSERT OR UPDATE OF resolved_at`
   вызывает `pg_notify('alert_events_channel', <payload>)` — одна триггер-функция на оба события,
   различаются полем `type` в payload.
3. Оба listener'а держат **выделенное** pgx-соединение каждый (не из общего pool — `LISTEN`
   привязан к конкретной сессии), в цикле блокируются на `conn.WaitForNotification(ctx)` — не
   polling, горутина спит до прихода уведомления по уже открытому соединению.
4. Оба пишут в общий `hub.go`, который рассылает всем подключённым клиентам через `client.go`.

Формат сообщения клиенту — единый конверт с полем `type`:

```json
{ "type": "reading", "data": { "sensor_id": "...", "value": 23.41, "time": "..." } }
{ "type": "alert_fired", "data": { "id": "...", "sensor_id": "...", "triggered_at": "..." } }
{ "type": "alert_resolved", "data": { "id": "...", "resolved_at": "..." } }
{ "type": "entity_changed", "data": { "entity": "controller", "action": "created", "id": "...", "...": "остальные поля строки" } }
```

Фронт различает по `type` (график — для `reading`, тост/бейдж — для `alert_*`); опциональные
`?sensor_id=`/`?controller_id=` при апгрейде — фильтрация подписки на стороне `client.go`, не
отдельные каналы.

**`entity_changed` — уведомления об изменениях `controllers`/`sensors`/`system_settings`**
(решение от 2026-08-09): оператор с открытым 24/7 терминалом должен узнавать о провижининге
нового оборудования/изменении настроек без обновления страницы. `data` — структурированные
`entity`/`action`/поля строки, **не** готовая строка сообщения от бэкенда: фронт сам решает, что
рендерить (текст тоста, точечный апдейт кэша списка вместо полного рефетча), исходя из
`entity`+`action`, а не парсит текст. Обсуждался вариант с плоским `{msg, level}` — отклонён:
теряет структуру для программной обработки на фронте и смешивает разную по природе ургентность
(алерт — бизнес-критичное срабатывание порога, провижининг — административное действие) под одним
полем `level`.

Источник этого события — **не `NOTIFY`/`LISTEN`**, в отличие от readings/alerts. `controllers`,
`sensors`, `system_settings` меняются только через сам API (`POST`/`PATCH`/`DELETE`, никогда
напрямую в обход бэкенда — см. scope-решения по провижинингу в `docs/PROJECT.md`), то есть писатель
и WS-хаб — один и тот же процесс. Поэтому соответствующий `handler` после успешного
`INSERT`/`UPDATE`/`DELETE` вызывает `hub.Broadcast(...)` **напрямую**, без похода через Postgres —
лишний `NOTIFY`-канал и `listener` были бы накладными расходами без причины. Если в будущем
появится второй писатель этих таблиц в обход API — придётся переезжать на `NOTIFY`-паттерн, как у
readings/alerts, не раньше.

**Владение (см. «Разделение работы» ниже):** `hub.go`/`client.go`/апгрейд-хендлер — общая
инфраструктура WS-слоя; `listener_readings.go` + миграция `000003` — один человек,
`listener_alerts.go` + миграция `000004` — другой, независимо, по идентичному паттерну.
`entity_changed` не требует отдельного listener'а — каждый вызывает `hub.Broadcast()` из своих же
handler'ов (Кирилл — из `controller_handler.go`/`sensor_handler.go`, друг — из `settings_handler.go`).

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
| `POST /auth/login`                                      | публично                                  | логин, выдача JWT (см. «Аутентификация: JWT-контракт»)                                          |
| `GET /controllers`, `GET /controllers/{id}`           | любая роль                               | чтение (список — с пагинацией, см. ниже)                                                              |
| `POST /controllers`, `PATCH/DELETE /controllers/{id}` | operator, admin                                   | провижининг устройств                                                                    |
| `GET /sensors?controller_id=`, `GET /sensors/{id}`    | любая роль                               | чтение; `controller_id` — опциональный фильтр (не вложенный роут, см. «Фильтрация `sensors`» ниже) |
| `POST /sensors`, `PATCH/DELETE /sensors/{id}`         | operator, admin                                   | провижининг устройств                                                                    |
| `GET /sensors/{id}/readings?from=&to=`                  | любая роль                               | история показаний для графика (raw/hourly — см. «Источник данных для `readings`» ниже)             |
| `GET /alerts?active=true`                               | любая роль                               | список алертов (пагинация)                                                                     |
| `PATCH /alerts/{id}/resolve`                            | operator, admin                                   | закрыть алерт                                                                                    |
| `GET /settings`, `GET /settings/{key}`                | любая роль                               | чтение системных настроек — см. «`system_settings` — маршруты» ниже                            |
| `PATCH /settings/{key}`                                 | admin                                             | изменение значения существующей настройки                                              |
| `GET /ws/metrics`                                       | любая роль (JWT при апгрейде) | единый realtime-канал: показания, алерты, изменения `controllers`/`sensors`/`settings` — см. «WebSocket: два потока событий, один канал» ниже |
| `POST /analytics/query`                                 | любая роль                               | LLM-сводка/анализ по естественному языку — см. «LLM Analytics» выше |

### `system_settings` — маршруты (решение зафиксировано)

Ключи создаются только через миграции/seed (`db/migrations/`), не через API — исключает
произвольное разрастание конфига мимо код-ревью и защищает от опечаток в имени ключа, которые
тихо создали бы новый, никем не читаемый параметр. Поэтому только `GET` (список и по ключу) и
`PATCH` (обновление `value` у существующего ключа); `POST`/`DELETE` не предусмотрены. `PATCH` —
только `admin` (глобальные настройки системы, не провижининг устройств — `operator` недостаточно).

```
GET   /settings           -> [{ "key": "...", "value": "...", "description": "...", "updated_at": "..." }]
GET   /settings/{key}     -> { "key": "...", "value": "...", "description": "...", "updated_at": "..." }
PATCH /settings/{key}     body: { "value": "..." }
```

### Фильтрация `sensors` по контроллеру

`GET /sensors?controller_id={id}` — query-параметр, не `GET /controllers/{id}/sensors`. Причина:
`sensors` уже самостоятельный ресурс с собственным `GET /sensors/{id}`; вложенный роут создал бы
два разных URL для одной и той же сущности (`/sensors/{id}` и `/controllers/{cid}/sensors/{id}`),
без реальной необходимости — сравнение фильтрации по FK через query-параметр стандартно и для
`sensors`, и в будущем для любого другого ресурса с похожей связью.

### Источник данных для `readings`

`GET /sensors/{id}/readings?from=&to=` выбирает таблицу автоматически по границе `from`,
привязанной к параметрам `continuous aggregate` (`add_continuous_aggregate_policy` в
`db/migrations/000001_init_schema.up.sql`: `start_offset => 3 days` — часовые бакеты младше 3
дней ещё не гарантированно материализованы):

- `from >= now() - 3 days` → `sensor_readings` (raw, гарантированно свежие данные)
- `from < now() - 3 days` → `sensor_readings_hourly` (continuous aggregate — сканировать 90 дней
  сырых точек ради графика бессмысленно и дорого)

Если диапазон пересекает границу — берётся `sensor_readings_hourly` целиком (упрощение для MVP,
единый источник на ответ вместо склейки raw+hourly). Без `from`/`to` — дефолт последние 24 часа
raw-данных (без дефолта случайный `GET .../readings` без параметров уронил бы либо полное
сканирование hypertable, либо потребовал 400 за отсутствие обязательных параметров — дефолт
дружелюбнее для UI). Верхняя граница диапазона не ограничивается отдельно: `sensor_readings`
физически не хранит данные старше 90 дней (retention policy), `sensor_readings_hourly` не
удаляется отдельной retention-политикой и может расти неограниченно — это осознанно (историчные
почасовые агрегаты дёшевы, удалять их отдельным решением не сейчас).

### Пагинация

Все списочные ручки (`GET /controllers`, `GET /sensors`, `GET /alerts`) принимают `?limit=&offset=`.
`limit` по умолчанию `50`, максимум `200` (запрос с `limit` больше — не ошибка, а тихий clamp до
200, чтобы не плодить лишние 400 на фронте). Ответ — обёртка вместо голого массива, чтобы клиент
всегда знал общее количество для постраничной навигации:

```json
{ "items": [...], "total": 137 }
```

### Формат ошибок

Единый JSON-контракт для всех non-2xx ответов (`handler/response.go`, `writeError`):

```json
{
  "error": {
    "code": "validation_error",
    "message": "человекочитаемое сообщение",
    "fields": { "min_threshold": "должен быть меньше max_threshold" }
  }
}
```

`fields` присутствует только при `code == "validation_error"` (результат
`go-playground/validator`, ключ — имя поля из DTO, значение — что не так). Соответствие
`code` ↔ HTTP-статусу фиксировано, обработчики на фронте могут переключаться по `code`, а не
парсить `message` (текст message может меняться, `code` — контракт):

| `code`              | HTTP статус |
|---------------------|-------------|
| `validation_error`  | 400         |
| `unauthorized`      | 401         |
| `forbidden`          | 403         |
| `not_found`          | 404         |
| `duplicate`          | 409         |
| `internal`           | 500         |

### Аутентификация: JWT-контракт

Claims: `sub` (user id, UUID), `role` (`admin`/`operator`/`viewer`), `iat`, `exp`. TTL — **24
часа**, без refresh-токена: для учебного пет-проекта с малым числом пользователей повторный логин
раз в сутки — не проблема UX, а refresh-flow (второй токен, ротация, хранение) — сложность без
пропорциональной пользы на этом этапе. Если появится реальная необходимость (например, долгие
WS-сессии, которые не должны рваться при истечении токена) — пересматривать отдельным решением,
не сейчас. `middleware/auth.go` кладёт `sub`/`role` в `context.Context` запроса,
`middleware/requirerole.go` читает роль оттуда.

### CORS

`middleware/cors.go` — allowlist origin'ов, не `*` (иначе теряется смысл авторизации через
cookie/заголовок для браузерных клиентов). Dev: `http://localhost:5173` (дефолтный порт Vite,
см. `ui/`). Prod origin — через переменную окружения `CORS_ALLOWED_ORIGINS` (ещё не заведена в
`.env.example`, добавить при деплое дашборда).

## Зависимости

Уже зафиксировано ранее в `docs/PROJECT.md`: `chi/v5` (роутер), `pgx/v5` + `pgxpool` (драйвер,
плюс `LISTEN`/`NOTIFY` для `ws/listener.go`), `sqlc` (кодогенерация, dev-time, не рантайм-зависимость
бинарника), `golang-jwt/v5` + `x/crypto/bcrypt` (auth), `go-playground/validator/v10` (валидация
DTO), `caarlos0/env/v11` (конфиг), `log/slog` (логи), `testify` (тесты).

Новое, зафиксированное этим документом: **`github.com/coder/websocket`** — WebSocket-библиотека
(бывш. `nhooyr.io/websocket`, активно поддерживается, в отличие от `gorilla/websocket`). Ранее в
`docs/PROJECT.md` был помечен как кандидат "если решится" — решено, WS реально проектируется.

**`github.com/go-chi/cors`** (2026-08-09, добавлена при реализации инфраструктурного каркаса) —
официальный компаньон-пакет `chi` для CORS (`middleware/cors.go`). Ручной разбор
preflight-запросов (`OPTIONS`, `Access-Control-*` заголовки) — с готовыми граблями (credentials +
`*` origin запрещены спецификацией, `Vary` заголовок и т.д.), доверять их пакету из той же
экосистемы, что и роутер, дешевле, чем писать и тестировать самому ради одного файла.

LLM Analytics не добавляет тяжёлых зависимостей — `internal/llm/client.go` реализован через
`net/http` к REST API выбранного провайдера, без официального SDK (см. «LLM-провайдер спрятан за
интерфейсом» выше).

## Разделение работы между Кириллом и другом (решение от 2026-08-09)

Оба участника учат Go на слое `api/` совместно (см. [[team-and-scope]] в памяти) — раньше
обсуждался разрез «по модулю с одинаковым паттерном», в итоге выбран более простой явный split по
предметным областям, без выравнивания объёма между участниками (осознанно принято, не пересмотр
без причины):

| Кому | Модули |
|---|---|
| Кирилл | `controllers`, `sensors`, `readings` (история показаний) + вся WS-инфраструктура (`hub.go`, `client.go`, апгрейд-хендлер `GET /ws/metrics`, `listener_readings.go` + миграция `000003`) |
| Друг | `auth` (`register`/`login`, JWT, bcrypt), `alerts`, `system_settings` + `listener_alerts.go` + миграция `000004` |

`internal/analytics`/`internal/llm` (LLM Analytics) — вне этого разделения, по «Ближайшим шагам» в
`docs/PROJECT.md` реализуется после того, как базовый CRUD заработает у обоих, не сейчас.

`config.go`, `main.go` (composition root), `router.go`, `pgxpool`, `sqlc.yaml` — общая
инфраструктура, неизбежно достаётся тому, кто пишет первый модуль (Кирилл, т.к. `controllers`
идёт первым по «Ближайшим шагам» в `docs/PROJECT.md`); друг подключает свои модули к уже
поднятой инфре, не поднимает её заново.

## Статус реализации

**Реализовано и проверено вживую (2026-08-09):**

- инфраструктурный каркас — `config.go`, `pgxpool` с fail-fast `Ping`, `router.go`,
  `middleware/logging.go`, `middleware/cors.go`, graceful shutdown;
- **модуль `controllers` целиком** — `model` → `repository` (sqlc) → `service` → `handler`,
  все пять ручек (`GET` список с пагинацией, `GET` по id, `POST`, `PATCH`, `DELETE`),
  формат ошибок и конверт `{items,total}` по контракту из этого документа.
- **модуль `sensors` целиком** (2026-08-09, написан через TDD) — те же пять ручек плюс
  необязательный фильтр `?controller_id=`. `metric_type` сознательно **не** ограничен списком
  значений: в схеме это `VARCHAR(50)` без `CHECK`, и API не должен быть строже схемы — новый
  тип метрики (vibration, current, flow) не должен требовать правки Go-кода и деплоя.

Проверено против реальной БД в Docker: пагинация и clamp, 409 на дубль `mqtt_gateway_id`,
400 с заполненным `fields` на невалидном теле и неизвестном поле, 404 на чужой/несуществующий
id, частичный `PATCH` (меняется только переданное поле), 204 на `DELETE` и 404 на повторный,
CORS-allowlist (разрешённый origin получает заголовки, посторонний — нет).

**Отклонение от плана:** RBAC на мутирующих ручках `controllers` не навешен — `auth`
(и, соответственно, `middleware/auth.go` + `requirerole.go`) в зоне друга и ещё не написан.
В `controller_handler.go` стоит `TODO(auth)` ровно там, куда встанет `RequireRole`.
До появления auth мутирующие ручки открыты — это осознанный временный компромисс, а не
недосмотр.

**Не реализовано:** `readings`, `ws/`, `analytics`/`llm`, а также все модули зоны друга.

## Тесты

Первый тестовый набор появился вместе с `sensors` (2026-08-09) — модуль писался через TDD:
сначала падающие тесты, потом код. Два уровня, разделённые по признаку «нужна ли БД»:

- **Юнит-тесты** (`internal/handler`, `internal/service`) — подделки интерфейсов, БД не
  задействована, бегут всегда: `make test`. Проверяют HTTP-контракт (коды ответов, конверт
  ошибок, `fields` с json-именами полей, clamp пагинации, разбор фильтра) и склейку
  списка с `total`.
- **Интеграционные** (`internal/repository`) — против настоящего Postgres, запускаются
  только при заданном `TEST_DB_DSN`, иначе `t.Skip`: `make test-integration`. Проверяют то,
  что подделкой не проверишь — перевод ошибок драйвера в доменные (`23503` →
  `ErrReferenceNotFound`, `23514` → `ErrConstraintViolation`, `23505` → `ErrDuplicate`) и
  корректность SQL (фильтр, частичный `UPDATE` через `COALESCE`).

**Изоляция интеграционных тестов:** каждый тест работает в своей транзакции с откатом в
`t.Cleanup`. Сгенерированный sqlc `DBTX` принимает и пул, и `pgx.Tx`, поэтому репозиторий
строится поверх транзакции — тесты не видят изменений друг друга, не зависят от порядка
выполнения и не оставляют мусор в БД. Ручная чистка не нужна.

**Двойная проверка порогов — сознательная, не дублирование по недосмотру.** `min < max`
проверяется и в Go (`handler/validation.go`), и `CHECK`-ом в схеме. Go-проверка срабатывает,
только когда переданы оба значения, и даёт точную привязку к полю формы
(`fields.min_threshold`). `CHECK` остаётся окончательной гарантией и покрывает случай, который
в Go проверить нечем: `PATCH` с одним порогом, когда второй лежит в базе. Оба пути закрыты
тестами.

## Тестовые данные

`db/seed.sql` + `make seed` — контроллеры, датчики и системные настройки для разработки.
Лежит в `db/`, но **не** в `db/migrations/`: это не миграция, в прод при `migrate up`
попадать не должно. И не в `api/` — Consumer'у нужны те же строки (без зарегистрированных
`controllers`/`sensors` он обязан дропать входящие MQTT-сообщения), поэтому seed — общая
инфраструктура по тому же принципу, что и миграции. Данные согласованы с топиками реального
симулятора (`gateway/esp32-01/{temperature,humidity,pressure}`), скрипт идемпотентен
(`ON CONFLICT DO NOTHING`).

## Что не в этом документе

Реализация (`.go`-файлы кроме уже существующего `cmd/api/main.go` с `/health`), сами SQL-триггеры
`000003`/`000004` (описан только их эффект — `pg_notify`, конкретный текст функции пишется вместе
с кодом listener'ов), сервис `api` в `docker-compose.yml` — следующие отдельные шаги, по одному
модулю за раз, начиная с `controllers`.
