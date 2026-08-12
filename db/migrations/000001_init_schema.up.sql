-- 0. Расширение TimescaleDB — обязательно ДО create_hypertable() ниже.
-- На образе timescale/timescaledb расширение доступно, но не активировано в БД по умолчанию.
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- 1. Пользователи и роли
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      VARCHAR(50) NOT NULL UNIQUE,
    email         VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role          VARCHAR(20) NOT NULL DEFAULT 'operator'
                  CHECK (role IN ('admin', 'operator', 'viewer')),
    created_at    TIMESTAMPTZ DEFAULT now(),
    updated_at    TIMESTAMPTZ DEFAULT now()
);

COMMENT ON TABLE  users IS 'Учётные записи, работающие с дашбордом и API';
COMMENT ON COLUMN users.id IS 'Первичный ключ';
COMMENT ON COLUMN users.username IS 'Логин пользователя, уникален';
COMMENT ON COLUMN users.email IS 'Почта пользователя, уникальна';
COMMENT ON COLUMN users.password_hash IS 'Хеш пароля (bcrypt), пароль в открытом виде не хранится';
COMMENT ON COLUMN users.role IS 'Роль доступа: admin — полный доступ, operator — управление датчиками/контроллерами, viewer — только просмотр';
COMMENT ON COLUMN users.created_at IS 'Дата создания учётной записи';
COMMENT ON COLUMN users.updated_at IS 'Дата последнего изменения записи';

-- 2. Контроллеры (ПЛК / Шлюзы / ESP32)
CREATE TABLE controllers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100) NOT NULL,
    mqtt_gateway_id VARCHAR(100) NOT NULL UNIQUE, -- gateway_id из топика gateway/{gateway_id}/{metric}
    ip_address      VARCHAR(45),
    location        VARCHAR(255),
    is_active       BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

COMMENT ON TABLE  controllers IS 'Физическое или виртуальное устройство (шлюз/ПЛК/ESP32), объединяющее один или несколько датчиков';
COMMENT ON COLUMN controllers.id IS 'Первичный ключ';
COMMENT ON COLUMN controllers.name IS 'Человекочитаемое имя контроллера для дашборда, можно менять без влияния на приём MQTT-данных';
COMMENT ON COLUMN controllers.mqtt_gateway_id IS 'gateway_id из MQTT-топика gateway/{gateway_id}/{metric} — по нему Consumer сопоставляет входящее сообщение с контроллером';
COMMENT ON COLUMN controllers.ip_address IS 'IP-адрес устройства на площадке (справочно)';
COMMENT ON COLUMN controllers.location IS 'Физическое расположение устройства (справочно)';
COMMENT ON COLUMN controllers.is_active IS 'Флаг активности контроллера';
COMMENT ON COLUMN controllers.created_at IS 'Дата создания записи';
COMMENT ON COLUMN controllers.updated_at IS 'Дата последнего изменения записи';

-- 3. Датчики (метрика и пороги — поля датчика, см. docs/database-schema.md «Известные проблемы»)
CREATE TABLE sensors (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    controller_id   UUID NOT NULL REFERENCES controllers(id) ON DELETE CASCADE,
    name            VARCHAR(100) NOT NULL,
    topic           VARCHAR(255) NOT NULL UNIQUE, -- MQTT topic
    metric_type     VARCHAR(50) NOT NULL,         -- 'temperature', 'humidity', 'pressure'
    unit            VARCHAR(20) NOT NULL,         -- '°C', 'Bar', '%'
    min_threshold   DOUBLE PRECISION,
    max_threshold   DOUBLE PRECISION,
    is_active       BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now(),
    CHECK (min_threshold IS NULL OR max_threshold IS NULL OR min_threshold < max_threshold)
);

CREATE INDEX idx_sensors_controller_id ON sensors (controller_id);

COMMENT ON TABLE  sensors IS 'Датчик, привязанный к контроллеру: MQTT-топик, тип метрики и пороги алертинга';
COMMENT ON COLUMN sensors.id IS 'Первичный ключ';
COMMENT ON COLUMN sensors.controller_id IS 'Контроллер, к которому физически подключён датчик — обязателен, датчик без контроллера не существует';
COMMENT ON COLUMN sensors.name IS 'Человекочитаемое имя датчика для дашборда';
COMMENT ON COLUMN sensors.topic IS 'Уникальный MQTT-топик, на который датчик публикует показания';
COMMENT ON COLUMN sensors.metric_type IS 'Тип измеряемой величины (temperature/humidity/pressure и т.д.), однозначно задан топиком';
COMMENT ON COLUMN sensors.unit IS 'Единица измерения (°C, Bar, % и т.д.)';
COMMENT ON COLUMN sensors.min_threshold IS 'Нижний порог значения для алертинга, NULL — порог не задан. Если оба порога заданы, min_threshold < max_threshold гарантировано CHECK';
COMMENT ON COLUMN sensors.max_threshold IS 'Верхний порог значения для алертинга, NULL — порог не задан';
COMMENT ON COLUMN sensors.is_active IS 'Флаг активности датчика';
COMMENT ON COLUMN sensors.created_at IS 'Дата создания записи';
COMMENT ON COLUMN sensors.updated_at IS 'Дата последнего изменения записи';

-- 4. Системные настройки (Key-Value для конфигураций)
CREATE TABLE system_settings (
    key         VARCHAR(100) PRIMARY KEY,
    value       TEXT NOT NULL,
    description TEXT,
    updated_at  TIMESTAMPTZ DEFAULT now()
);

COMMENT ON TABLE  system_settings IS 'Глобальные настройки системы в формате key-value (интервалы опроса, флаги функциональности и т.д.)';
COMMENT ON COLUMN system_settings.key IS 'Уникальный ключ настройки, первичный ключ таблицы';
COMMENT ON COLUMN system_settings.value IS 'Значение настройки (хранится как текст, парсится на уровне приложения)';
COMMENT ON COLUMN system_settings.description IS 'Пояснение назначения настройки';
COMMENT ON COLUMN system_settings.updated_at IS 'Дата последнего изменения значения';

-- 5. Временные ряды (Телеметрия) — TimescaleDB Hypertable
CREATE TABLE sensor_readings (
    time        TIMESTAMPTZ       NOT NULL,
    sensor_id   UUID              NOT NULL REFERENCES sensors(id) ON DELETE CASCADE,
    value       DOUBLE PRECISION  NOT NULL,
    PRIMARY KEY (sensor_id, time)
);

COMMENT ON TABLE  sensor_readings IS 'Hypertable TimescaleDB с показаниями датчиков, партиционирование по времени';
COMMENT ON COLUMN sensor_readings.time IS 'Момент измерения (partitioning-колонка hypertable), входит в PRIMARY KEY';
COMMENT ON COLUMN sensor_readings.sensor_id IS 'Датчик, к которому относится показание';
COMMENT ON COLUMN sensor_readings.value IS 'Измеренное значение в единице измерения датчика (sensors.unit)';

SELECT create_hypertable('sensor_readings', 'time');
CREATE INDEX idx_sensor_readings_time ON sensor_readings (sensor_id, time DESC);

-- Retention: хранить сырые данные 90 дней (плейсхолдер, см. docs/database-schema.md «Известные проблемы»)
SELECT add_retention_policy('sensor_readings', INTERVAL '90 days');

-- Continuous aggregate: часовые агрегаты для дашборда и долгих графиков
CREATE MATERIALIZED VIEW sensor_readings_hourly
WITH (timescaledb.continuous) AS
SELECT
    sensor_id,
    time_bucket('1 hour', time) AS bucket,
    avg(value) AS avg_value,
    min(value) AS min_value,
    max(value) AS max_value
FROM sensor_readings
GROUP BY sensor_id, bucket;

SELECT add_continuous_aggregate_policy('sensor_readings_hourly',
    start_offset => INTERVAL '3 days',
    end_offset   => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour');

-- 6. История срабатываний порогов алертинга
CREATE TABLE alert_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sensor_id       UUID NOT NULL REFERENCES sensors(id) ON DELETE CASCADE,
    threshold_type  VARCHAR(10) NOT NULL CHECK (threshold_type IN ('min', 'max')),
    threshold_value DOUBLE PRECISION NOT NULL,
    reading_value   DOUBLE PRECISION NOT NULL,
    triggered_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at     TIMESTAMPTZ -- NULL, пока значение остаётся вне порога
);

CREATE INDEX idx_alert_events_sensor_triggered ON alert_events (sensor_id, triggered_at DESC);
CREATE INDEX idx_alert_events_active ON alert_events (sensor_id) WHERE resolved_at IS NULL;

COMMENT ON TABLE  alert_events IS 'История срабатываний порогов алертинга (sensors.min_threshold/max_threshold)';
COMMENT ON COLUMN alert_events.id IS 'Первичный ключ';
COMMENT ON COLUMN alert_events.sensor_id IS 'Датчик, чьё показание вышло за порог';
COMMENT ON COLUMN alert_events.threshold_type IS 'Какой порог нарушен: min или max';
COMMENT ON COLUMN alert_events.threshold_value IS 'Значение порога на момент срабатывания (снимок sensors.min_threshold/max_threshold, независим от последующих правок порога)';
COMMENT ON COLUMN alert_events.reading_value IS 'Значение показания, вызвавшее срабатывание';
COMMENT ON COLUMN alert_events.triggered_at IS 'Момент срабатывания алерта';
COMMENT ON COLUMN alert_events.resolved_at IS 'Момент возврата значения в допустимый диапазон, NULL — алерт ещё активен';
