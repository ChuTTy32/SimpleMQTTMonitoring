# Industrial Sensor Monitor (SimpleMQTTMonitoring)

Учебно-боевой IIoT pet-проект: путь от симулированного промышленного датчика до
веб-дашборда — MQTT → Docker → TimescaleDB → .NET → веб-интерфейс.

**Таск-трекер (Jira):** https://chutty.atlassian.net/browse/KAN-1

## Что это

Система мониторинга показаний промышленного датчика в реальном времени: симулятор
генерирует данные (температура, влажность, давление), публикует их по MQTT, бэкенд
принимает и складывает в базу временных рядов, REST API отдаёт данные, дашборд
отображает живой график.

```
[Fake Sensor (Python)]
        │  MQTT publish (topic: gateway/{gateway_id}/{metric_type})
        ▼
[Mosquitto Broker (Docker)]
        │  MQTT subscribe
        ▼
[Backend Consumer]            — план, кода нет (только consumer/.gitkeep)
        │  INSERT
        ▼
[PostgreSQL + TimescaleDB]    — план, сервиса нет в docker-compose.yml
        │  SELECT
        ▼
[REST API (+ WebSocket?)]     — план, кода нет (только api/.gitkeep)
        ▼
[Vue Dashboard]                — каркас есть в ui/, к бэкенду не подключён
```

Целиком должно подниматься одной командой `docker compose up` — сейчас так поднимаются
только брокер и симулятор, см. «Как запустить» ниже.

Подробный и постоянно актуализируемый статус по каждому модулю, стеку, схеме БД и
известным расхождениям с планом — в [`docs/PROJECT.md`](docs/PROJECT.md). Этот README —
входная точка и краткая выжимка, а не источник правды.

## Зачем

Проект — фундамент для:

1. Боевого IIoT-проекта в магистратуре
2. Первой Scopus-публикации по промышленному мониторингу
3. Портфолио для cold email профессорам KAIST / POSTECH
4. GKS University Track (дедлайн февраль 2028)

Архитектура ориентируется на Auto-ID Lab KAIST (Connectivity Layer → Middleware →
Storage → Application), только вместо трекинга товаров — промышленная автоматизация.

Вторичная цель — оба участника команды понимают систему целиком, а не только свой слой.

## Команда

- **Кирилл** — технический лид, архитектура, фронтенд
- **Друг** — бэкенд (.NET, PostgreSQL, инфраструктура)

## Версия

`0.1.0` — ранняя стадия активной разработки, git-теги не проставлены. Актуальный статус
по модулям — в [`docs/PROJECT.md`](docs/PROJECT.md#статус-по-модулям).

## Стек

По каждому модулю — кратко, детали и точные версии в [`docs/PROJECT.md`](docs/PROJECT.md):

- **Fake Sensor** (`sensor/`) — Python, paho-mqtt, python-dotenv
- **MQTT Broker** (`mosquitto/`) — Mosquitto 2.x (Docker)
- **Consumer** (`consumer/`) — .NET Worker Service + MQTTnet + Npgsql *(запланировано; директория есть, кода нет)*
- **PostgreSQL + TimescaleDB** *(схема описана в [`docs/database-schema.md`](docs/database-schema.md), сервис ещё не развёрнут)*
- **REST API** (`api/`) — .NET Minimal API, возможно + WebSocket для realtime *(запланировано; директория есть, кода нет)*
- **Дашборд** (`ui/`) — Vue 3 + TypeScript + Vite + Pinia + Vue Router + Axios + Tailwind CSS *(каркас есть, к реальному бэкенду не подключён)*
- **Инфраструктура** — Docker + Docker Compose

## Структура репозитория

```
├── docker-compose.yml                 # оркестрация сервисов (пока: broker + simulator)
├── SimpleMQTTMonitoring.code-workspace # multi-root workspace для VS Code
├── AGENTS.md                          # инструкции для AI-агентов, работающих в репозитории
├── README.MD                          # этот файл
├── mosquitto/                         # конфигурация MQTT-брокера
├── sensor/                            # Fake Sensor — симулятор датчика (Python)
├── consumer/                          # Backend Consumer — только .gitkeep, кода нет
├── api/                                # REST API — только .gitkeep, кода нет
├── ui/                                 # Дашборд (Vue 3 + Vite) — каркас, не подключён к бэкенду
└── docs/
    ├── PROJECT.md                      # живой контекст проекта — источник правды
    ├── database-schema.md              # схема БД PostgreSQL + TimescaleDB
    ├── ROADMAP-SKILL.md                # скилл генерации карты кода (docs/codemap/)
    └── codemap/                        # генерируется roadmap-скиллом, не редактируется вручную
```

## Формат данных датчиков

Fake Sensor публикует показания в топики вида `gateway/{gateway_id}/{metric_type}`, где
`metric_type` — `temperature`, `humidity` или `pressure`:

```json
{
  "value": 23.41,
  "timestamp": "2026-07-31T10:00:00Z"
}
```

## Как запустить

Сейчас end-to-end пайплайн не собран (нет БД/consumer/API/дашборда в работающем виде) —
рабочая часть на сегодня: MQTT-брокер + симулятор датчика.

**Известная проблема:** `sensor/Dockerfile` копирует `.env-example` как `.env` при сборке
образа, а текущий `sensor/.env-example` содержит placeholder-значения (`MQTT_BROKER_PORT=99999`
— невалидный порт, `MQTT_BROKER=some-hostname`), не реальные дефолты. Из коробки
`docker compose up --build` соберёт нерабочий контейнер симулятора — перед сборкой создайте
`sensor/.env` с реальными значениями (см. пример ниже).

```bash
git clone <repo-url>
cd SimpleMQTTMonitoring
cat > sensor/.env <<'EOF'
MQTT_BROKER=mqtt-broker
MQTT_BROKER_PORT=1883
GATEWAY_ID=esp32-01
PUBLISH_INTERVAL_SEC=5
EOF
docker compose up --build
```

Проверить, что данные идут, можно подпиской на топики (пример через `mosquitto_sub`,
если он установлен локально):

```bash
mosquitto_sub -h localhost -p 1883 -t "gateway/#" -v
```

За полным и актуальным статусом того, что уже поднимается, а что ещё нет — см.
[`docs/PROJECT.md`](docs/PROJECT.md).

## Разработка в VS Code

В корне репозитория есть `SimpleMQTTMonitoring.code-workspace` — открывайте через него,
чтобы получить отдельные корни для `sensor`, `consumer`, `api`, `ui`, `mosquitto` и `docs`
в одном окне.

## Дополнительные ссылки

- **Jira:** https://chutty.atlassian.net/browse/KAN-1
- **Контекст проекта (живой документ):** [`docs/PROJECT.md`](docs/PROJECT.md)
- **Схема БД:** [`docs/database-schema.md`](docs/database-schema.md)
- **Карта кода (генерируется скиллом):** `docs/codemap/codemap.html`
