# Industrial Sensor Monitor — общее описание проекта

Учебно-боевой IIoT pet-проект, команда из двух человек. Цель — освоить промышленный стек
(MQTT → Docker → TimescaleDB → .NET → веб-дашборд) и заложить фундамент для научной
публикации по теме IIoT. Вторичная цель — оба участника понимают всю систему целиком,
а не только свой слой.

Этот файл — единая точка правды по проекту: архитектура, стек, статус модулей, схема БД,
формат MQTT-сообщений, состояние веток. Обновляется по мере продвижения (в отличие от
исходного плана на старте проекта, который зафиксировал модули 0-7 и процесс работы —
его содержание консолидировано сюда и в задачи Jira).

**Таск-трекер (Jira):** https://chutty.atlassian.net/browse/KAN-1

## Целевая архитектура

```
[Fake Sensor (Python)]
        |
        | MQTT publish (topic: gateway/{gateway_id}/{metric})
        v
[Mosquitto Broker (Docker)]
        |
        | MQTT subscribe
        v
[.NET Consumer (Worker Service)]      <- ещё не реализован (только consumer/.gitkeep)
        |
        | INSERT
        v
[PostgreSQL + TimescaleDB (Docker)]   <- сервис в docker-compose.yml, живьём не проверен
        |
        | SELECT
        v
[REST API (Go)]                       <- ещё не реализован (только api/.gitkeep)
        |
        | HTTP polling (возможно + WebSocket, см. ниже)
        v
[Web Dashboard (Vue 3 + Vite)]        <- каркас в main/ui/, не подключён к бэкенду
```

Вся система должна подниматься одной командой: `docker compose up`. Сейчас так поднимаются
`mqtt-broker`, `simulator`, `db` (TimescaleDB) и `migrate` (одноразовый прогон схемы) — сервисов
для consumer/API/дашборда в `docker-compose.yml` ещё нет.

**`Makefile`** (2026-08-08) — обёртка над `docker compose` для повседневных команд: `make up`,
`make down`, `make ps`, `make logs` / `make logs-one SERVICE=db`, `make migrate-up` /
`make migrate-down` / `make migrate-version`, `make db-shell` (psql внутрь `db`). `make` без
аргументов — список команд (`make help`). Переменные окружения подхватываются из корневого
`.env`.

## Статус по модулям (факт на 2026-08-08)

| # | Модуль | Статус | Комментарий |
|---|--------|--------|-------------|
| 0 | Подготовка | ✅ сделано | Репозиторий, README, ветки |
| 1 | Docker + Mosquitto | ✅ сделано | Брокер поднимается в Docker, топики рабочие |
| 2 | PostgreSQL + TimescaleDB | ✅ сделано, проверено вживую (2026-08-09) | `db` + `migrate` реально поднимаются, обе миграции применяются (`exit 0`), hypertable/continuous aggregate/роль `analytics_readonly` созданы. Потребовало починки миграций — см. «Миграции: найденные при первом живом запуске проблемы» ниже |
| 3 | Fake Sensor | ✅ сделано | Публикует 3 метрики; `sensor/Dockerfile` собирается из локального `sensor/.env` (gitignored) — см. «Известные проблемы» про `.env-example` как шаблон |
| 4 | .NET Consumer | ⬜ не начато | Директория `consumer/` создана (только `.gitkeep`), кода нет |
| 5 | REST API | 🟡 инфраструктура + `controllers` + `sensors` работают вживую | Каркас (config/pgxpool/router/CORS/graceful shutdown) и полный CRUD `controllers`/`sensors` (с фильтром `?controller_id=`) проверены против реальной БД; заглушка `GET /health` снесена. Есть тесты: `make test` / `make test-integration`. Остальное (`readings`, `ws`, зона друга) не реализовано — см. [`docs/api-architecture.md`](api-architecture.md), разделы «Статус реализации» и «Тесты» |
| 6 | Веб-дашборд | 🟡 каркас в main, не подключён | Стек — Vue 3 + Vite (не Nuxt, см. ниже); смёржен в `main`, но не подключён к реальному API |
| 7 | Финал / интеграция | ⬜ не начато | End-to-end пайплайн (датчик → БД → API → дашборд) пока не собран |
| 8 | LLM Analytics | 🟡 архитектура спроектирована, кода нет | `POST /analytics/query`, tool-calling цикл, read-only роль `analytics_readonly` — см. «LLM Analytics» ниже и [`docs/api-architecture.md`](api-architecture.md) |

## Миграции: найденные при первом живом запуске проблемы (2026-08-09)

Первый реальный `docker compose up db migrate` (до этого compose проверялся только
статически через `docker compose config`) вскрыл три бага, из-за которых схема не
разворачивалась вообще. Все исправлены, миграции проходят с `exit 0`:

1. **`CREATE MATERIALIZED VIEW ... WITH DATA cannot run inside a transaction block.**
   TimescaleDB запрещает создавать continuous aggregate внутри транзакции, а golang-migrate
   без доп. флага отправляет весь файл одним simple query — Postgres оборачивает такой пакет
   в неявную транзакцию. Решение: флаг **`x-multi-statement=true`** в строке подключения
   (`docker-compose.yml` и `DB_URL` в `Makefile`) — операторы выполняются по одному.
2. **`COMMENT ON MATERIALIZED VIEW` на continuous aggregate падает.** В TimescaleDB
   continuous aggregate — это обычная view поверх внутренней материализованной гипертаблицы,
   в системном каталоге числится как view. Исправлено на `COMMENT ON VIEW`.
3. **Наивное разбиение по «;».** Плата за `x-multi-statement`: golang-migrate режет файл по
   точке с запятой, не понимая ни строк, ни комментариев. Ломались `--` комментарии с «;»
   в прозе, `COMMENT ON ... IS '...;...'` и `DO $$ ... $$` блок. Точки с запятой из прозы
   убраны, DO-блок заменён (`GRANT CONNECT` не нужен — он есть у `PUBLIC` по умолчанию).

Правила и симптомы вынесены в **[`db/migrations/README.md`](../db/migrations/README.md)** —
читать перед написанием новой миграции. Главное: **никаких «;» кроме конца оператора**, и
никаких PL/pgSQL-блоков. Второе следствие флага — миграции больше не атомарны: упавший
на середине файл оставляет часть операторов применёнными и версию `dirty`, лечится
`make down-v && make up` (для dev-базы приемлемо).

## Известные проблемы

- **`sensor/Dockerfile` теперь собирается через `COPY .env .env`** — берёт `.env` из
  контекста сборки (`.dockerignore` в `sensor/` нет, файл не отфильтруется), а не копирует
  `.env-example` как раньше. Это правильное направление: заставляет разработчика явно
  положить рабочий `sensor/.env` перед сборкой, а не тянуть плейсхолдеры. Локально
  `sensor/.env` уже содержит рабочие значения (`MQTT_BROKER=mqtt-broker`,
  `MQTT_BROKER_PORT=1883`). **Не проверено реальной сборкой** — Docker недоступен в
  окружении, где это проверялось, только статически по синтаксису.
- **`sensor/.env-example` по-прежнему вводит в заблуждение.** Содержит явные
  placeholder-значения: `MQTT_BROKER=some-hostname`, `MQTT_BROKER_PORT=99999` (невалидный
  порт, > 65535), `GATEWAY_ID=controller-name-or-id`, `PUBLISH_INTERVAL_SEC=60`. Раньше это
  ломало сборку напрямую (файл копировался как `.env`); сейчас Dockerfile его не трогает,
  но если разработчик по привычке сделает `cp .env-example .env`, получит нерабочие
  значения. Стоит либо поправить `.env-example` на реалистичный шаблон, либо явно
  задокументировать в README, что копировать его один в один нельзя.

## Технологический стек (факт)

### Fake Sensor — `sensor/`

- Python 3.10 (Dockerfile: `python:3.10-slim`; в исходном плане был 3.12)
- `paho-mqtt==1.6.1`, `python-dotenv==1.0.1`
- Конфигурация через `.env` (`MQTT_BROKER`, `MQTT_BROKER_PORT`, `GATEWAY_ID`, `PUBLISH_INTERVAL_SEC`)
- Генерирует три метрики с шумом и синусоидальной динамикой: температура, влажность, давление
- См. «Известные проблемы» — `.env-example` сейчас не рабочий

### MQTT Broker — `mosquitto/`

- `eclipse-mosquitto:2.0`
- `mosquitto.conf`: `listener 1883`, `allow_anonymous true`, кастомный `log_timestamp_format`

### PostgreSQL + TimescaleDB

- Сервис `db` (`timescale/timescaledb:latest-pg16`) и `migrate` (`migrate/migrate:v4.17.1`)
  добавлены в `docker-compose.yml` (2026-08-08). Миграция — `db/migrations/000001_init_schema.up/down.sql`,
  зеркало SQL-блока из `docs/database-schema.md` (см. раздел «Миграции БД» ниже за архитектурой
  разделения migrate/api/consumer).
- Конфиг — через корневой `.env` (`POSTGRES_USER`/`POSTGRES_PASSWORD`/`POSTGRES_DB`), шаблон —
  `.env.example`, оба с рабочими dev-значениями (не мусорными плейсхолдерами, в отличие от
  прежней проблемы с `sensor/.env-example` — см. «Известные проблемы»). `.env` в `.gitignore`.
- `docker compose config` проходит без ошибок (синтаксис/интерполяция переменных валидны), но
  реальный `docker compose up` не проверялся — Docker недоступен в среде, где это писалось.

### .NET Consumer — `consumer/`

- Не начат. В плане: .NET 8 Worker Service + MQTTnet, подписка на `gateway/+/+`, запись
  в `sensor_readings`. Директория создана (`consumer/.gitkeep`), исходников нет.

### REST API — `api/`

- **Решение о смене стека (2026-08-08):** REST API переведён с .NET Minimal API на Go.
  Consumer (`consumer/`) остаётся на .NET — смена стека касается только слоя API,
  явно ограничена пользователем формулировкой «веб-апи».
- Первый шаг сделан (2026-08-08): `GET /health` на голом `net/http`, без роутера/слоёв — учебная
  ручка, чтобы было к чему делать запрос. Полная структура (CRUD-модули + WebSocket-канал метрик,
  дерево каталогов, маршруты, зависимости) спроектирована и задокументирована отдельно —
  [`docs/api-architecture.md`](api-architecture.md). Реализация остальных модулей — по одному за
  раз. Стек:
  - **Go 1.23**
  - **chi v5** — роутер. Лёгкий, идиоматичный net/http-совместимый роутинг с middleware —
    ближайший Go-аналог по духу к .NET Minimal API, с которым уже был план.
  - **pgx v5** (`jackc/pgx`) + `pgxpool` — драйвер PostgreSQL/TimescaleDB, нативный протокол,
    без cgo.
  - **sqlc** — генерация типобезопасных Go-структур и функций из SQL-запросов
    (`db/migrations/` как источник схемы, см. «Миграции БД» ниже). Соответствует правилу
    проекта «нет типа — напиши интерфейс»: никакого `interface{}`/сырых `map[string]any` от БД.
  - **golang-jwt/jwt/v5** + `golang.org/x/crypto/bcrypt` — аутентификация и роли
    (`users.role`: `admin`/`operator`/`viewer` уже есть в схеме).
  - **go-playground/validator/v10** — валидация входных DTO на границе API.
  - **log/slog** (стандартная библиотека) — структурированное логирование.
  - **caarlos0/env/v11** — типизированный конфиг из переменных окружения (без Viper —
    оверкилл для одного сервиса).
  - **testify** + `net/http/httptest` — юнит- и HTTP-тесты; `testcontainers-go` —
    опционально для интеграционных тестов с реальным Postgres.
  - WebSocket для realtime — **решено** (2026-08-08): `github.com/coder/websocket` (бывш.
    `nhooyr.io/websocket`, активно поддерживается, в отличие от `gorilla/websocket`), данные —
    через Postgres `LISTEN`/`NOTIFY` (Consumer и API не связаны напрямую, общая точка — БД, тот же
    принцип, что и для миграций). Детали — [`docs/api-architecture.md`](api-architecture.md).
  - Layout (слоистая архитектура, как и на фронте) — полное дерево каталогов, маршруты и разбор
    WS-канала вынесены в отдельный документ, см. [`docs/api-architecture.md`](api-architecture.md),
    чтобы не дублировать и не расходиться с ним.
  - **Архитектура доведена до полного состояния (2026-08-09):** закрыты пробелы, которые не
    позволяли начать реализацию — маршруты `system_settings`, фильтрация `sensors` по
    `controller_id`, пагинация списочных ручек, единый формат ошибок, JWT-контракт (TTL 24ч, без
    refresh), CORS-allowlist, источник данных (raw/hourly) для `GET .../readings`, и решение
    сделать WS одним каналом с двумя `NOTIFY`-listener'ами вместо двух физических эндпоинтов —
    все детали в `api-architecture.md`.

### LLM Analytics — `api/internal/analytics`, `api/internal/llm`

**Решение зафиксировано (2026-08-09):** пользователь задаёт вопрос на естественном языке
(«как вела себя температура датчика X за неделю», «сравни контроллеры 1 и 2») и получает
текстовую сводку. Архитектура — минимально достаточная, часть исходной концепции сознательно
отклонена на этом этапе:

- **не отдельный микросервис** — модуль `internal/analytics` внутри уже существующего `api/`,
  не `llm-analytics/` со своим Dockerfile/compose-сервисом;
- **не Python-sandbox с произвольной генерацией кода** — отдельный security-проект
  непропорционального риска для текущего масштаба команды (см. [[team-and-scope]] в памяти);
  если появится задача, для которой не хватает конечного набора tool'ов — решать отдельно, не
  сейчас;
- **не отдельная таблица `llm_context`** — контекст (какие есть контроллеры/датчики) вычисляется
  `SELECT`-ом на лету при каждом запросе, не материализуется.

**Главный принцип:** LLM — это парсер естественного языка в аргументы функций (tool calling),
не более. Вся логика доступа к данным — детерминированный Go + SQL. Полное описание
tool-calling цикла, набора инструментов, лимитов и read-only доступа к БД —
[`docs/api-architecture.md`](api-architecture.md), раздел «LLM Analytics».

**Правки БД (2026-08-09, `db/migrations/000002_llm_analytics_readonly.up/down.sql`):**
`sensor_readings_hourly` (continuous aggregate) пересоздан с добавлением `stddev_value` и
`reading_count` (ALTER для continuous aggregate недоступен в TimescaleDB — только пересоздание);
добавлена read-only роль `analytics_readonly` с `GRANT SELECT` только на `controllers`,
`sensors`, `sensor_readings`, `sensor_readings_hourly` — `users`, `alert_events`,
`system_settings` осознанно исключены, пока под них нет tool'а. Переменные окружения —
`ANALYTICS_DB_USER`/`ANALYTICS_DB_PASSWORD`/`LLM_API_KEY` в `.env`/`.env.example`.

Из-за этой миграции будущая `NOTIFY`-триггер миграция для WS-канала (см. «REST API — `api/`»
выше) сдвинута с `000002` на `000003`.

### Веб-дашборд — `ui/` (в `main`, не подключён к бэкенду)

Фактический стек **разошёлся с исходным планом**: вместо Nuxt 4 + Nuxt UI + vue-chartjs
команда завела проект на:

- Vue 3.5 + Vite 8 (не Nuxt)
- vue-router 5, Pinia 4
- Tailwind CSS 4
- axios (HTTP-клиент, не встроенный Nuxt `$fetch`)
- @vueuse/core, @iconify/vue
- Package name: `mqtt-simple-control-panel`
- Тесты: Vitest + @vue/test-utils
- Линтинг: ESLint + oxlint + Prettier

Каркас содержит `App.vue`, `main.ts`, `router/index.ts`, `stores/counter.ts` (заглушка
Pinia), компоненты `Layout/AppScaffold.vue`, `Sidebar/SideBar.vue`, страницу
`pages/MainPage.vue`. Реальных данных с бэкенда пока не подключено — axios есть в
зависимостях, но ни одного вызова к API в коде нет.

## MQTT: топики и формат сообщений (факт)

Отличается от исходного плана (`sensors/temperature`, payload с `sensor_id`).

**Топик:** `gateway/{GATEWAY_ID}/{metric}`, например `gateway/esp32-01/temperature`,
`gateway/esp32-01/humidity`, `gateway/esp32-01/pressure`.

**Payload (JSON):**

```json
{
  "value": 23.41,
  "timestamp": "2026-08-06T12:34:56Z"
}
```

`sensor_id`/`gateway_id` в payload не передаётся — идентификатор шлюза зашит в топик.
QoS = 0. Интервал публикации управляется `PUBLISH_INTERVAL_SEC`.

## Схема БД (спроектирована, не развёрнута)

Полное описание и SQL — [`docs/database-schema.md`](database-schema.md) (в `main`).
PostgreSQL + TimescaleDB, `sensor_readings` — hypertable.

**Правка от 2026-08-08** (архитектурное решение, зафиксировано по итогам ревью схемы):
таблица `metrics` убрана (`metric_type`/пороги алертинга перенесены в `sensors` — при старом
дизайне `sensor_readings` не могла определить, какой метрике принадлежит значение, см.
«Известные проблемы» в `database-schema.md`); `controllers` получил `mqtt_gateway_id` для
сопоставления с `gateway_id` из MQTT-топика; `users.role` теперь ограничен `CHECK`;
`sensor_readings` получил `PRIMARY KEY (sensor_id, time)` от дублей; добавлены retention policy
и continuous aggregate (часовые агрегаты) для `sensor_readings` — без них TimescaleDB
использовалась бы как обычный Postgres; `sensors.controller_id` стал `NOT NULL` с индексом
(FK-колонки не индексируются автоматически), пороги защищены `CHECK (min_threshold < max_threshold)`;
добавлена таблица `alert_events` — история срабатываний порогов. Все поля таблиц задокументированы
`COMMENT ON COLUMN` прямо в SQL. Детали — в `database-schema.md`.

| Таблица | Назначение |
|---|---|
| `users` | Пользователи и роли (`admin`, `operator`, `viewer`) |
| `controllers` | Физические контроллеры/шлюзы (ПЛК, ESP32 и т.п.), с `mqtt_gateway_id` |
| `sensors` | Датчики, привязанные к контроллеру: MQTT-топик, метрика и пороги алертинга |
| `system_settings` | Глобальные настройки в формате key-value |
| `sensor_readings` | Временные ряды показаний (TimescaleDB hypertable, retention + continuous aggregate) |
| `alert_events` | История срабатываний порогов алертинга |

**Решение о провижининге устройств (2026-08-08):** система не plug-and-play — новый
контроллер/датчик должен быть зарегистрирован вручную (админом/оператором через дашборд или
REST API — `POST /controllers`, `POST /sensors` с `name`/`topic`/`mqtt_gateway_id`/`metric_type`
и т.д.) **до** того, как Consumer сможет принять его показания. Причина: `sensors.controller_id`
`NOT NULL`, `sensors.topic` и `controllers.mqtt_gateway_id` — `UNIQUE`, `sensor_readings.sensor_id`
— `NOT NULL REFERENCES` — писать некуда, если строк ещё нет. Consumer при сообщении с неизвестным
`gateway_id`/`topic` **не создаёт** запись автоматически — дропает сообщение и логирует
(WARN-уровень, с топиком и payload для диагностики), чтобы MQTT не был открытым каналом
регистрации устройств без контроля доступа. Автопровижининг (даже с ручным подтверждением через
`pending`-статус) явно отклонён на этом этапе — решение зафиксировано здесь, чтобы не
пересматривалось implicit-но при написании Consumer.

Эта схема заметно более развитая, чем плоская таблица `sensor_readings` из исходного плана
(модуль 2) — добавлены сущности `users`, `controllers`, `sensors` для
многодатчиковой/мультиконтроллерной системы с ролями доступа. Текущий Fake Sensor (топик
вида `gateway/{id}/{metric}`) уже спроектирован с расчётом на эту схему (топик хранится в
`sensors.topic`, `gateway_id` — в `controllers.mqtt_gateway_id`).

## Миграции БД: независимая инфраструктура, не часть `api/` (решение от 2026-08-08)

Изначально миграции планировались как часть Go-стека REST API (`golang-migrate` внутри `api/`).
Это архитектурная ошибка: Consumer (.NET) тоже пишет в ту же БД, но не имеет отношения к тому,
как и когда применяются миграции API-сервиса — получается скрытая, неявная зависимость Consumer
от деплоя/раннего запуска API. Схема БД — общая инфраструктура, а не собственность одного из
бэкенд-сервисов.

**Правильная модель:**

- **`db/migrations/`** — каталог в корне репозитория (рядом с `api/`, `consumer/`, `sensor/`),
  единственный источник правды по схеме. Содержимое — версионированные `.sql`-файлы в конвенции
  `golang-migrate` (`{версия}_{название}.up.sql` / `.down.sql`). Первая миграция
  `000001_init_schema.up/down.sql` создана и зеркалит SQL-блок из `docs/database-schema.md`.
- **Отдельный one-shot сервис `migrate` в `docker-compose.yml`** — официальный образ
  `migrate/migrate` (CLI `golang-migrate`, не библиотека, вшитая в Go-код API), применяет
  `db/migrations/` к поднятой БД и завершается (`exit 0`).
- **И `consumer`, и `api` объявляют `depends_on: migrate` с `condition: service_completed_successfully`**
  (штатная фича Docker Compose для init/миграционных контейнеров) — оба сервиса стартуют только
  после успешной миграции, оба зависят от неё одинаково, ни один её не «владеет».

**Что остаётся внутри `api/`:** только **sqlc** — кодогенерация типизированных Go-структур из
SQL-запросов *для самого API*. Это не миграции, а инструмент конкретного сервиса: Consumer читает
ту же схему своими средствами (Dapper/EF Core), никак не завязан на sqlc. Кодогенерация —
особенность реализации одного сервиса, миграции — общая инфраструктура; смешивать их в один слой
не стоит.

## Структура репозитория (main, факт)

```
├── docker-compose.yml                  # mqtt-broker, simulator, db, migrate
├── Makefile                             # обёртка над docker compose, см. выше
├── .env / .env.example                  # POSTGRES_USER/PASSWORD/DB, .env в .gitignore
├── SimpleMQTTMonitoring.code-workspace  # multi-root workspace для VS Code
├── AGENTS.md
├── README.MD
├── mosquitto/config/mosquitto.conf
├── sensor/                              # Fake Sensor (Python) — см. «Известные проблемы»
│   ├── simulator.py
│   ├── Dockerfile
│   ├── requirements.txt
│   └── .env-example
├── db/
│   ├── seed.sql                         # тестовые данные для dev (`make seed`), НЕ миграция
│   └── migrations/                      # независимая инфраструктура схемы БД, см. «Миграции БД»
│       ├── README.md                    # ⚠️ правила написания миграций (никаких «;» в прозе)
│       ├── 000001_init_schema.up.sql    # не часть api/ или consumer/
│       ├── 000001_init_schema.down.sql
│       ├── 000002_llm_analytics_readonly.up.sql   # stddev/count + роль analytics_readonly
│       └── 000002_llm_analytics_readonly.down.sql
├── consumer/                            # только .gitkeep, кода нет
├── api/                                 # Go REST API: каркас + модуль controllers (рабочий)
│   ├── cmd/api/main.go                  # composition root
│   ├── internal/{config,model,repository,service,handler,middleware}/
│   ├── db/queries/                      # SQL-источник для sqlc
│   └── sqlc.yaml
├── ui/                                  # Vue 3 + Vite дашборд, каркас
└── docs/
    ├── PROJECT.md                       # этот файл
    ├── database-schema.md               # схема БД PostgreSQL + TimescaleDB
    ├── api-architecture.md              # структура api/: CRUD-модули, WS-канал, зависимости
    ├── image/database-schema/           # диаграмма к схеме
    ├── ROADMAP-SKILL.md                 # скилл генерации карты кода
    └── codemap/                         # генерируется roadmap-скиллом, не редактируется вручную
```

## Ветки

`feature/ui` смёржена в `main` через PR #1 (2026-08-06) — дашборд, схема БД и заглушки
`api/`/`consumer/` теперь в основной ветке. Отдельного `feature/ui` больше не требуется
для получения этого кода (ветка может ещё существовать в удалённом репозитории — при
уборке веток проверить, что она безопасно удаляется после мержа).

## Дальнейшие цели проекта

Проект — фундамент для:

1. Боевого IIoT-проекта в магистратуре
2. Первой Scopus-публикации по промышленному мониторингу
3. Портфолио для cold email профессорам KAIST / POSTECH
4. GKS University Track (дедлайн февраль 2028)

Архитектура ориентируется на Auto-ID Lab KAIST (Connectivity Layer → Middleware → Storage →
Application), с промышленной автоматизацией вместо трекинга товаров.

## Scope: только мониторинг, без управления (решение от 2026-08-08)

Система read-only по отношению к устройствам: принимает и отображает телеметрию, но **не
отправляет команды** датчикам/контроллерам/актуаторам (реле и т.п.). Вся текущая схема
(`sensors`, `sensor_readings`, `metric_type` + пороги) спроектирована под одностороннюю
телеметрию — она не рассчитана на актуаторы без переработки.

Управление устройствами (например, реле) явно вынесено в бэклог — не проектируется и не
закладывается в схему/API сейчас. Если появится, потребует отдельно: конвенцию MQTT-топиков для
команд (отдельно от телеметрии, QoS 1), таблицу команд/desired-state с аудитом (кто/когда/что
отправил), более строгую авторизацию под запись в устройство (не `viewer`), топик
`{gateway_id}/{actuator}/state` для подтверждения фактического состояния. Решать по факту, когда
появится реальная потребность — не проектировать заранее под гипотетический сценарий.

## Команда

**Решение о разделении бэкенд-стека (2026-08-08):** Consumer остаётся на .NET, REST API — на Go
(см. раздел «REST API — `api/`»). Стеки сознательно не сведены к одному — Go на слое API оба
участника команды изучают совместно, а не только Кирилл в одиночку.

- Кирилл — технический лид, архитектура, фронтенд
- Друг — бэкенд (.NET/Consumer, PostgreSQL, инфраструктура, низкоуровенное программирование);
  Go на REST API изучает вместе с Кириллом
- Оба — полное понимание всей системы, не только своего слоя

**Разделение работы над `api/` (2026-08-09):** зафиксировано в `docs/api-architecture.md`,
раздел «Разделение работы между Кириллом и другом» — Кирилл: `controllers`/`sensors`/`readings` +
WS-инфраструктура и `listener_readings.go`; друг: `auth`/`alerts`/`system_settings` и
`listener_alerts.go`. Split неравномерный по объёму (Кирилл берёт три модуля и всю
WS-инфраструктуру, включая её как первый, кто пишет `main.go`/`router.go`/`pgxpool`) — осознанное
решение пользователя, не пересматривать без явного запроса.

## Ближайшие шаги (вытекают из статуса выше)

1. ~~Проверить `docker compose up` реальной сборкой, включая сервисы `db`/`migrate`~~ — сделано
   (2026-08-09): `db` + `migrate` подняты живьём, миграции применяются, потребовалась починка
   (см. «Миграции: найденные при первом живом запуске проблемы»). **Осталось:** `sensor/` живьём
   так и не собирался — `sensor/Dockerfile` делает `COPY .env .env`, а `sensor/.env` отсутствует
   (в `.gitignore`), поэтому `docker compose up` целиком сейчас падает на сборке `simulator`;
   поднимать приходится точечно (`docker compose up -d db migrate`). Поправить `.env-example`
   на реалистичный шаблон и завести локальный `sensor/.env`
2. ~~Добавить сервис PostgreSQL+TimescaleDB в `docker-compose.yml`~~ — сделано (2026-08-08):
   `db` + `migrate` в compose, `db/migrations/000001_init_schema.up/down.sql`. Осталось только
   проверить живым запуском (см. п.1)
3. Начать .NET Consumer: подписка на `gateway/+/+`, запись в `sensor_readings`; `depends_on:
   migrate` с `condition: service_completed_successfully`; **не создавать** `controllers`/`sensors`
   автоматически на неизвестный `gateway_id`/`topic` — дропать и логировать (см. «Решение о
   провижининге устройств» выше)
4. REST API на Go (chi + pgx + sqlc): ~~инфраструктура + модули `controllers` и `sensors`~~ —
   сделано и проверено вживую (2026-08-09), `sensors` написан через TDD и покрыт тестами.
   ~~Решить HTTP polling vs WebSocket~~ — решено, WS одним каналом
   (см. `api-architecture.md`). **Дальше по зоне Кирилла:** `readings`
   (`GET /sensors/{id}/readings`, выбор raw/hourly по границе `from`), затем `ws/` (хаб +
   listener + миграция `000003`). Зона друга
   (`auth`, `alerts`, `system_settings`) не начата — из-за этого RBAC на мутирующих ручках
   `controllers` пока отсутствует, стоит `TODO(auth)`. Ещё не сделано: `api/Dockerfile` и сервис
   `api` в `docker-compose.yml` с `depends_on: migrate`
5. Подключить дашборд к реальным данным вместо заглушек
6. Реализовать `internal/analytics`/`internal/llm` (см. «LLM Analytics» выше) — после того как
   базовый CRUD `api/` заработает; миграция `000002_llm_analytics_readonly.up.sql` уже применяется
   вместе с `000001` через сервис `migrate`, реализации кода пока нет
