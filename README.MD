# Industrial Sensor Monitor

Учебно-практический IIoT-проект: полный цикл от физического датчика до веб-дашборда в реальном времени.

## Цель проекта

Построить систему мониторинга промышленных датчиков (температура, влажность, давление и т.д.):
симулированный датчик публикует данные по MQTT → брокер принимает сообщения → бэкенд сохраняет их
в базу данных временных рядов → REST API и WebSocket отдают данные на фронтенд → веб-дашборд
отображает живые графики показаний и статусы контроллеров.

## Архитектура

```
[Fake Sensor (Python)]
        │  MQTT publish (topic: gateway/{gateway_id}/{metric_type})
        ▼
[Mosquitto Broker (Docker)]
        │  MQTT subscribe
        ▼
[Backend Consumer]
        │  INSERT
        ▼
[PostgreSQL + TimescaleDB]
        │  SELECT
        ▼
[REST API + WebSocket]
        │  HTTP-запросы + realtime push
        ▼
[Vue Dashboard]
```

Брокер и симулятор поднимаются одной командой: `docker compose up`.

## Стек

### Fake Sensor (`sensor/`)
- Python 3.12
- paho-mqtt — публикация сообщений в MQTT-брокер
- python-dotenv — конфигурация через `.env`

### MQTT Broker (`mosquitto/`)
- Mosquitto 2.x (`eclipse-mosquitto`, Docker)

### База данных
- PostgreSQL + TimescaleDB — хранение телеметрии в виде hypertable
- Полная схема БД описана в [`docs/database-schema.md`](docs/database-schema.md)

### Frontend (`ui/`)
- Vue 3 + TypeScript
- Pinia — управление состоянием
- Axios — HTTP-запросы к REST API
- WebSocket — realtime-обновления показаний датчиков
- Tailwind CSS — стилизация
- Vue Router — маршрутизация

### Инфраструктура
- Docker + Docker Compose
- `.env` — конфигурация через переменные окружения (в репозитории не хранится)

## Структура репозитория

```
SimpleMQTTMonitoring/
├── docker-compose.yml               — поднимает брокер и симулятор
├── SimpleMQTTMonitoring.code-workspace
├── mosquitto/                       — конфигурация MQTT-брокера
│   └── config/mosquitto.conf
├── sensor/                          — Fake Sensor (Python)
│   ├── Dockerfile
│   ├── simulator.py
│   └── requirements.txt
├── consumer/                         — Backend Consumer (MQTT → БД)
├── api/                               — REST API + WebSocket
├── ui/                               — Frontend (Vue 3 + TS)
└── docs/                             — документация проекта
    └── database-schema.md
```

## Формат данных датчиков

Fake Sensor публикует показания в топики вида `gateway/{gateway_id}/{metric_type}`, где
`metric_type` — `temperature`, `humidity` или `pressure`. Формат сообщения:

```json
{
  "value": 23.41,
  "timestamp": "2026-07-31T10:00:00Z"
}
```

## Быстрый старт

```bash
docker compose up
```

Поднимет Mosquitto-брокер и симулятор датчика. Симулятор публикует данные каждые
`PUBLISH_INTERVAL_SEC` секунд (по умолчанию 2), см. `sensor/.env-example`.

## Разработка в VS Code

В корне репозитория есть `SimpleMQTTMonitoring.code-workspace` — открывайте через него, чтобы
получить отдельные корни для `sensor`, `consumer`, `api`, `ui`, `mosquitto` и `docs` в одном окне.
