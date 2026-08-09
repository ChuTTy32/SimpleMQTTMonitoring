-- Continuous aggregate нельзя ALTER — TimescaleDB требует пересоздания при изменении
-- набора агрегатных колонок. Добавляем stddev и count, нужные LLM Analytics для сводок
-- (docs/api-architecture.md, раздел «LLM Analytics»): stddev — индикатор нестабильности
-- датчика, count — sanity-check, что в периоде вообще были показания.
DROP MATERIALIZED VIEW IF EXISTS sensor_readings_hourly;

CREATE MATERIALIZED VIEW sensor_readings_hourly
WITH (timescaledb.continuous) AS
SELECT
    sensor_id,
    time_bucket('1 hour', time) AS bucket,
    avg(value)    AS avg_value,
    min(value)    AS min_value,
    max(value)    AS max_value,
    stddev(value) AS stddev_value,
    count(*)      AS reading_count
FROM sensor_readings
GROUP BY sensor_id, bucket;

SELECT add_continuous_aggregate_policy('sensor_readings_hourly',
    start_offset => INTERVAL '3 days',
    end_offset   => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour');

COMMENT ON MATERIALIZED VIEW sensor_readings_hourly IS 'Часовые агрегаты sensor_readings: используются дашбордом для длинных графиков и LLM Analytics для сводок без обращения к сырым данным';
COMMENT ON COLUMN sensor_readings_hourly.stddev_value IS 'Стандартное отклонение показаний за час — индикатор нестабильности датчика для LLM Analytics';
COMMENT ON COLUMN sensor_readings_hourly.reading_count IS 'Количество показаний в часовом бакете';

-- Read-only роль для LLM Analytics (api/internal/analytics, см. docs/api-architecture.md).
-- LLM-инструменты (get_sensor_summary и т.д.) обязаны физически не иметь возможности
-- писать в БД, даже если весь SQL в коде — SELECT: это защита от бага в Go-коде, а не
-- только от непредсказуемости LLM. Доступ дан точечно — только под существующие
-- инструменты; users (хеши паролей), alert_events и system_settings исключены осознанно,
-- расширять GRANT только вместе с новым tool'ом, не заранее.
--
-- Пароль — dev-значение по конвенции .env.example (см. POSTGRES_PASSWORD), не боевой
-- секрет; при деплое в прод — сменить через ALTER ROLE, миграция не должна нести реальный
-- пароль.
CREATE ROLE analytics_readonly WITH LOGIN PASSWORD 'analytics_readonly_local_dev';

DO $$
BEGIN
    EXECUTE format('GRANT CONNECT ON DATABASE %I TO analytics_readonly', current_database());
END $$;

GRANT USAGE ON SCHEMA public TO analytics_readonly;
GRANT SELECT ON controllers, sensors, sensor_readings, sensor_readings_hourly TO analytics_readonly;
