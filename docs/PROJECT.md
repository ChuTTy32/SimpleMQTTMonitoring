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
[PostgreSQL + TimescaleDB (Docker)]   <- ещё не развёрнут (схема описана)
        |
        | SELECT
        v
[REST API (Go)]                       <- ещё не реализован (только api/.gitkeep)
        |
        | HTTP polling (возможно + WebSocket, см. ниже)
        v
[Web Dashboard (Vue 3 + Vite)]        <- каркас в main/ui/, не подключён к бэкенду
```

Вся система должна подниматься одной командой: `docker compose up`. Сейчас так
поднимаются только `mqtt-broker` и `simulator` — сервисов для БД/consumer/API/дашборда
в `docker-compose.yml` ещё нет.

## Статус по модулям (факт на 2026-08-06, после мержа `feature/ui` → `main`)

| # | Модуль | Статус | Комментарий |
|---|--------|--------|-------------|
| 0 | Подготовка | ✅ сделано | Репозиторий, README, ветки |
| 1 | Docker + Mosquitto | ✅ сделано | Брокер поднимается в Docker, топики рабочие |
| 2 | PostgreSQL + TimescaleDB | 🟡 спроектировано | Схема БД в `docs/database-schema.md` (в `main`), сервис в `docker-compose.yml` ещё не добавлен |
| 3 | Fake Sensor | ✅ сделано | Публикует 3 метрики; `sensor/Dockerfile` собирается из локального `sensor/.env` (gitignored) — см. «Известные проблемы» про `.env-example` как шаблон |
| 4 | .NET Consumer | ⬜ не начато | Директория `consumer/` создана (только `.gitkeep`), кода нет |
| 5 | REST API | ⬜ не начато | Директория `api/` создана (только `.gitkeep`), кода нет |
| 6 | Веб-дашборд | 🟡 каркас в main, не подключён | Стек — Vue 3 + Vite (не Nuxt, см. ниже); смёржен в `main`, но не подключён к реальному API |
| 7 | Финал / интеграция | ⬜ не начато | End-to-end пайплайн (датчик → БД → API → дашборд) пока не собран |

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

- Не развёрнут. Схема спроектирована и задокументирована в `docs/database-schema.md`
  (теперь в `main`, см. раздел «Схема БД» ниже), но сервиса в `docker-compose.yml` пока нет.

### .NET Consumer — `consumer/`

- Не начат. В плане: .NET 8 Worker Service + MQTTnet, подписка на `gateway/+/+`, запись
  в `sensor_readings`. Директория создана (`consumer/.gitkeep`), исходников нет.

### REST API — `api/`

- **Решение о смене стека (2026-08-08):** REST API переведён с .NET Minimal API на Go.
  Consumer (`consumer/`) остаётся на .NET — смена стека касается только слоя API,
  явно ограничена пользователем формулировкой «веб-апи».
- Не начат. Директория создана (`api/.gitkeep`), исходников нет. Стек:
  - **Go 1.23**
  - **chi v5** — роутер. Лёгкий, идиоматичный net/http-совместимый роутинг с middleware —
    ближайший Go-аналог по духу к .NET Minimal API, с которым уже был план.
  - **pgx v5** (`jackc/pgx`) + `pgxpool` — драйвер PostgreSQL/TimescaleDB, нативный протокол,
    без cgo.
  - **sqlc** — генерация типобезопасных Go-структур и функций из SQL-запросов
    (`docs/database-schema.md` как источник схемы). Соответствует правилу проекта
    «нет типа — напиши интерфейс»: никакого `interface{}`/сырых `map[string]any` от БД.
  - **golang-migrate/migrate** — версионированные миграции поверх SQL из
    `docs/database-schema.md`.
  - **golang-jwt/jwt/v5** + `golang.org/x/crypto/bcrypt` — аутентификация и роли
    (`users.role`: `admin`/`operator`/`viewer` уже есть в схеме).
  - **go-playground/validator/v10** — валидация входных DTO на границе API.
  - **log/slog** (стандартная библиотека) — структурированное логирование.
  - **caarlos0/env/v11** — типизированный конфиг из переменных окружения (без Viper —
    оверкилл для одного сервиса).
  - **testify** + `net/http/httptest` — юнит- и HTTP-тесты; `testcontainers-go` —
    опционально для интеграционных тестов с реальным Postgres.
  - WebSocket для realtime — по-прежнему **не решено** (см. предыдущую формулировку про
    `feature/ui`); если решится — кандидат `coder/websocket` (бывш. `nhooyr.io/websocket`,
    активно поддерживается, в отличие от `gorilla/websocket`).
  - Layout (слоистая архитектура, как и на фронте):
    ```
    api/
    ├── cmd/api/main.go
    ├── internal/
    │   ├── config/
    │   ├── handler/       # HTTP-хендлеры, роуты chi
    │   ├── service/       # бизнес-логика
    │   ├── repository/    # sqlc-сгенерированный доступ к БД
    │   ├── model/          # DTO / доменные типы
    │   └── middleware/     # auth, логирование, CORS
    ├── migrations/          # golang-migrate SQL
    ├── go.mod
    └── Dockerfile           # multi-stage: golang:1.23-alpine → distroless
    ```

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

| Таблица | Назначение |
|---|---|
| `users` | Пользователи и роли (`admin`, `operator`, `viewer`) |
| `controllers` | Физические контроллеры/шлюзы (ПЛК, ESP32 и т.п.) |
| `sensors` | Датчики, привязанные к контроллеру, с уникальным MQTT-топиком |
| `metrics` | Типы измеряемых величин датчика и пороги для алертинга |
| `system_settings` | Глобальные настройки в формате key-value |
| `sensor_readings` | Временные ряды показаний (TimescaleDB hypertable, партиционирование по времени) |

Эта схема заметно более развитая, чем плоская таблица `sensor_readings` из исходного плана
(модуль 2) — добавлены сущности `users`, `controllers`, `sensors`, `metrics` для
многодатчиковой/мультиконтроллерной системы с ролями доступа. Текущий Fake Sensor (топик
вида `gateway/{id}/{metric}`) уже спроектирован с расчётом на эту схему (топик хранится в
`sensors.topic`).

## Структура репозитория (main, факт)

```
├── docker-compose.yml                  # только mqtt-broker + simulator
├── SimpleMQTTMonitoring.code-workspace  # multi-root workspace для VS Code
├── AGENTS.md
├── README.MD
├── mosquitto/config/mosquitto.conf
├── sensor/                              # Fake Sensor (Python) — см. «Известные проблемы»
│   ├── simulator.py
│   ├── Dockerfile
│   ├── requirements.txt
│   └── .env-example
├── consumer/                            # только .gitkeep, кода нет
├── api/                                 # только .gitkeep, кода нет
├── ui/                                  # Vue 3 + Vite дашборд, каркас
└── docs/
    ├── PROJECT.md                       # этот файл
    ├── database-schema.md               # схема БД PostgreSQL + TimescaleDB
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

## Команда

- Кирилл — технический лид, архитектура, фронтенд
- Друг — бэкенд (.NET, PostgreSQL, инфраструктура, низкоуровенное программирование)
- Оба — полное понимание всей системы, не только своего слоя

## Ближайшие шаги (вытекают из статуса выше)

1. Проверить `docker compose up --build` реальной сборкой (не проверено — Docker недоступен в среде, где это писалось) и поправить `sensor/.env-example` на реалистичный шаблон
2. Добавить сервис PostgreSQL+TimescaleDB в `docker-compose.yml`, применить схему из `database-schema.md`
3. Начать .NET Consumer: подписка на `gateway/+/+`, запись в `sensor_readings`
4. Начать REST API на Go (chi + pgx + sqlc, см. раздел «REST API — `api/`» выше);
   решить HTTP polling vs WebSocket, задокументировать решение здесь
5. Подключить дашборд к реальным данным вместо заглушек
