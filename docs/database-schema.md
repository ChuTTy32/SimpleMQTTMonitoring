# Схема базы данных

База данных — PostgreSQL с расширением TimescaleDB. Телеметрия хранится в hypertable
`sensor_readings`, остальные таблицы — обычные реляционные таблицы с настройками системы.

## Обзор таблиц

| Таблица             | Назначение                                                        |
|----------------------|--------------------------------------------------------------------|
| `users`              | Пользователи системы и их роли (`admin`, `operator`, `viewer`)    |
| `controllers`        | Физические контроллеры/шлюзы (ПЛК, ESP32 и т.п.)                 |
| `sensors`             | Датчики, привязанные к контроллеру, с MQTT-топиком               |
| `metrics`             | Типы измеряемых величин датчика и пороговые значения              |
| `system_settings`    | Глобальные настройки системы в формате key-value                  |
| `sensor_readings`    | Временные ряды показаний датчиков (TimescaleDB hypertable)        |

### `users` — пользователи и роли

Учётные записи, работающие с дашбордом и API. Роль определяет уровень доступа:
`admin` — полный доступ, `operator` — управление датчиками/контроллерами,
`viewer` — только просмотр.

### `controllers` — контроллеры (ПЛК / шлюзы / ESP32)

Физическое или виртуальное устройство, объединяющее один или несколько датчиков
(например, шлюз на объекте). `ip_address` и `location` — служебная информация для
идентификации устройства на площадке.

### `sensors` — датчики

Конкретный датчик, привязанный к контроллеру. `topic` — уникальный MQTT-топик, на который
датчик публикует показания, `unit` — единица измерения (`°C`, `Bar`, `%` и т.д.).

### `metrics` — измеряемые величины и пороги

Тип метрики, которую отдаёт датчик (`temperature`, `humidity`, `pressure` и т.д.), вместе с
допустимым диапазоном (`min_threshold` / `max_threshold`) для алертинга на дашборде.

### `system_settings` — системные настройки

Произвольные конфигурационные параметры системы в виде пар ключ-значение (интервалы опроса,
флаги функциональности и т.д.), чтобы не хардкодить их в коде.

### `sensor_readings` — временные ряды (телеметрия)

Hypertable TimescaleDB с фактическими показаниями датчиков. Партиционируется по времени
автоматически. Индекс `idx_sensor_readings_time` ускоряет типичный запрос "последние N
показаний конкретного датчика".

## SQL

```sql
-- 1. Пользователи и роли
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      VARCHAR(50) NOT NULL UNIQUE,
    email         VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role          VARCHAR(20) NOT NULL DEFAULT 'operator', -- 'admin', 'operator', 'viewer'
    created_at    TIMESTAMPTZ DEFAULT now()
);

-- 2. Контроллеры (ПЛК / Шлюзы / ESP32)
CREATE TABLE controllers (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL,
    ip_address  VARCHAR(45),
    location    VARCHAR(255),
    is_active   BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now()
);

-- 3. Датчики
CREATE TABLE sensors (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    controller_id   UUID REFERENCES controllers(id) ON DELETE CASCADE,
    name            VARCHAR(100) NOT NULL,
    topic           VARCHAR(255) NOT NULL UNIQUE, -- MQTT topic
    unit            VARCHAR(20) NOT NULL,         -- '°C', 'Bar', '%'
    is_active       BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- 4. Метрики (Типы измеряемых величин и пороговые значения)
CREATE TABLE metrics (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sensor_id     UUID REFERENCES sensors(id) ON DELETE CASCADE,
    metric_type   VARCHAR(50) NOT NULL, -- 'temperature', 'humidity', 'pressure'
    min_threshold DOUBLE PRECISION,
    max_threshold DOUBLE PRECISION,
    created_at    TIMESTAMPTZ DEFAULT now()
);

-- 5. Системные настройки (Key-Value для конфигураций)
CREATE TABLE system_settings (
    key         VARCHAR(100) PRIMARY KEY,
    value       TEXT NOT NULL,
    description TEXT,
    updated_at  TIMESTAMPTZ DEFAULT now()
);

-- 6. Временные ряды (Телеметрия) — TimescaleDB Hypertable
CREATE TABLE sensor_readings (
    time        TIMESTAMPTZ       NOT NULL,
    sensor_id   UUID              NOT NULL REFERENCES sensors(id) ON DELETE CASCADE,
    value       DOUBLE PRECISION  NOT NULL
);

SELECT create_hypertable('sensor_readings', 'time');
CREATE INDEX idx_sensor_readings_time ON sensor_readings (sensor_id, time DESC);
```
