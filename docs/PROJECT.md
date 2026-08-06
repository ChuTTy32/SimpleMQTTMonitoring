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
[.NET Consumer (Worker Service)]      <- ещё не реализован
        |
        | INSERT
        v
[PostgreSQL + TimescaleDB (Docker)]   <- ещё не развёрнут
        |
        | SELECT
        v
[.NET REST API (Minimal API)]         <- ещё не реализован
        |
        | HTTP polling
        v
[Web Dashboard (Vue 3 + Vite)]        <- в разработке на ветке feature/ui
```

Вся система должна подниматься одной командой: `docker compose up`.

## Статус по модулям (факт на 2026-08-06)

| #   | Модуль                   | Статус                        | Комментарий                                                                                                           |
| --- | ------------------------ | ----------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| 0   | Подготовка               | ✅ сделано                    | Репозиторий, README, ветки                                                                                            |
| 1   | Docker + Mosquitto       | ✅ сделано                    | Брокер поднимается в Docker, топики рабочие                                                                           |
| 2   | PostgreSQL + TimescaleDB | 🟡 спроектировано             | Схема БД описана в`docs/database-schema.md` (ветка `feature/ui`), сервис в `docker-compose.yml` ещё не добавлен       |
| 3   | Fake Sensor              | ✅ сделано, диверг. от плана  | Публикует 3 метрики (temperature/humidity/pressure), формат топика и payload отличается от исходного плана (см. ниже) |
| 4   | .NET Consumer            | ⬜ не начато                  | Директории`consumer/` в дереве нет                                                                                    |
| 5   | REST API                 | ⬜ не начато                  | Директории`api/` в дереве нет                                                                                         |
| 6   | Веб-дашборд              | 🟡 в работе, диверг. от плана | Стек сменился с Nuxt 4 на Vue 3 + Vite (см. ниже); каркас проекта — только на ветке`feature/ui`, в `main` не смёржен  |
| 7   | Финал / интеграция       | ⬜ не начато                  | End-to-end пайплайн (датчик → БД → API → дашборд) пока не собран                                                      |

## Технологический стек (факт)

### Fake Sensor — `sensor/`

- Python 3.10 (Dockerfile: `python:3.10-slim`; в исходном плане был 3.12)
- `paho-mqtt==1.6.1`, `python-dotenv==1.0.1`
- Конфигурация через `.env` (`MQTT_BROKER`, `MQTT_BROKER_PORT`, `GATEWAY_ID`, `PUBLISH_INTERVAL_SEC`)
- Генерирует три метрики с шумом и синусоидальной динамикой: температура, влажность, давление

### MQTT Broker — `mosquitto/`

- `eclipse-mosquitto:2.0`
- `mosquitto.conf`: `listener 1883`, `allow_anonymous true`, кастомный `log_timestamp_format`

### PostgreSQL + TimescaleDB

- Не развёрнут. Схема спроектирована и задокументирована (см. раздел «Схема БД» ниже),
  но сервиса в `docker-compose.yml` пока нет.

### .NET Consumer / REST API

- Не начаты. В плане: .NET 8 Worker Service + MQTTnet для консьюмера, Minimal API + общий
  `Shared.Models` проект для REST API. Директории `consumer/` и `api/` в текущем дереве
  отсутствуют, несмотря на коммит `b319e2c "Make dir's for api, consumer, dashboard, docs"` —
  видимо, пустые директории не попали в git (git не трекает пустые папки).

### Веб-дашборд — `ui/` (ветка `feature/ui`, не в `main`)

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

Каркас содержит `App.vue`, `main.ts`, `router/index.ts`, `stores/counter.ts` (заглушка Pinia),
компоненты `Layout/AppScaffold.vue`, `Sidebar/SideBar.vue`, страницу `pages/MainPage.vue`.
Реальных данных с бэкенда пока не подключено.

В `main` папки `ui/` сейчас нет вообще (ранее там лежал пустой untracked-каркас без файлов —
он убран). Реальный код дашборда существует только на ветке `feature/ui`. Стоит решить,
сливать ли `feature/ui` в `main`, чтобы дашборд не оставался изолированным.

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
QoS = 0. Интервал публикации управляется `PUBLISH_INTERVAL_SEC` (по умолчанию 5с).

## Схема БД (спроектирована, не развёрнута)

Полное описание и SQL — `docs/database-schema.md` (сейчас только на ветке `feature/ui`).
PostgreSQL + TimescaleDB, `sensor_readings` — hypertable.

| Таблица           | Назначение                                                                      |
| ----------------- | ------------------------------------------------------------------------------- |
| `users`           | Пользователи и роли (`admin`, `operator`, `viewer`)                             |
| `controllers`     | Физические контроллеры/шлюзы (ПЛК, ESP32 и т.п.)                                |
| `sensors`         | Датчики, привязанные к контроллеру, с уникальным MQTT-топиком                   |
| `metrics`         | Типы измеряемых величин датчика и пороги для алертинга                          |
| `system_settings` | Глобальные настройки в формате key-value                                        |
| `sensor_readings` | Временные ряды показаний (TimescaleDB hypertable, партиционирование по времени) |

Эта схема заметно более развитая, чем плоская таблица `sensor_readings` из исходного плана
(модуль 2) — добавлены сущности `users`, `controllers`, `sensors`, `metrics` для
многодатчиковой/мультиконтроллерной системы с ролями доступа. Текущий Fake Sensor (топик
вида `gateway/{id}/{metric}`) уже спроектирован с расчётом на эту схему (топик хранится в
`sensors.topic`).

## Структура репозитория (main, факт)

```
├── docker-compose.yml          # только mqtt-broker + simulator
├── mosquitto/config/mosquitto.conf
├── sensor/                     # Fake Sensor (Python)
│   ├── simulator.py
│   ├── Dockerfile
│   ├── requirements.txt
│   └── .env-example
└── docs/
    ├── PROJECT.md                # этот файл
    └── ROADMAP-SKILL.md          # скилл генерации карты кода (docs/codemap/)
```

На ветке `feature/ui` дополнительно: полноценный Vue-проект в `ui/` и `docs/database-schema.md`.

## Ветки

- `main` — актуальная история: брокер + симулятор. Отстаёт от `feature/ui` на 3 коммита.
- `feature/ui` — содержит `main` целиком + каркас Vue-дашборда + схему БД. Не смёржена.

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

1. Решить, когда сливать `feature/ui` в `main` — дашборд пока изолирован на отдельной ветке
2. Добавить сервис PostgreSQL+TimescaleDB в `docker-compose.yml`, применить схему из `database-schema.md`
3. Начать .NET Consumer: подписка на `gateway/+/+`, запись в `sensor_readings`
4. Начать REST API поверх БД
5. Подключить дашборд к реальным данным вместо заглушек
