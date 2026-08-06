# Industrial Sensor Monitor (SimpleMQTTMonitoring)

Учебно-боевой IIoT pet-проект: путь от симулированного промышленного датчика до
веб-дашборда — MQTT → Docker → TimescaleDB → .NET → веб-интерфейс.

**Таск-трекер (Jira):** https://chutty.atlassian.net/browse/KAN-1

## Что это

Система мониторинга показаний промышленного датчика в реальном времени: симулятор
генерирует данные (температура, влажность, давление), публикует их по MQTT, бэкенд
принимает и складывает в базу временных рядов, REST API отдаёт данные, дашборд
отображает живой график. Целиком поднимается одной командой `docker compose up`.

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

- **Fake Sensor** — Python, paho-mqtt, python-dotenv
- **MQTT Broker** — Mosquitto 2.x (Docker)
- **Consumer** — .NET Worker Service + MQTTnet + Npgsql *(запланировано, ещё не реализовано)*
- **PostgreSQL + TimescaleDB** *(спроектировано, ещё не развёрнуто)*
- **REST API** — .NET Minimal API *(запланировано, ещё не реализовано)*
- **Дашборд** — Vue 3 + Vite + Pinia + Tailwind CSS *(в разработке, ветка `feature/ui`)*
- **Инфраструктура** — Docker + Docker Compose

## Структура репозитория

```
├── docker-compose.yml     # оркестрация сервисов
├── AGENTS.md              # инструкции для AI-агентов, работающих в репозитории
├── README.MD              # этот файл
├── mosquitto/              # конфигурация MQTT-брокера
├── sensor/                 # Fake Sensor — симулятор датчика (Python)
├── docs/
│   ├── PROJECT.md          # живой контекст проекта — источник правды
│   ├── ROADMAP-SKILL.md    # определение скилла генерации карты кода (docs/codemap/)
│   └── codemap/            # генерируется roadmap-скиллом, не редактируется вручную
└── ui/                     # веб-дашборд
```

## Как запустить

Сейчас end-to-end пайплайн ещё не собран (нет БД/consumer/API/дашборда в `main`) — рабочая
часть на сегодня: MQTT-брокер + симулятор датчика.

```bash
git clone <repo-url>
cd SimpleMQTTMonitoring
cp sensor/.env-example sensor/.env   # при необходимости поправить параметры
docker compose up --build
```

Проверить, что данные идут, можно подпиской на топики (пример через `mosquitto_sub`,
если он установлен локально):

```bash
mosquitto_sub -h localhost -p 1883 -t "gateway/#" -v
```

За полным и актуальным статусом того, что уже поднимается, а что ещё нет — см.
[`docs/PROJECT.md`](docs/PROJECT.md).

## Дополнительные ссылки

- **Jira:** https://chutty.atlassian.net/browse/KAN-1
- **Контекст проекта (живой документ):** [`docs/PROJECT.md`](docs/PROJECT.md)
- **Карта кода (генерируется скиллом):** `docs/codemap/codemap.html`
