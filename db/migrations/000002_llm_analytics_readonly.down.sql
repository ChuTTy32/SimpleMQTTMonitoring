REVOKE SELECT ON controllers, sensors, sensor_readings, sensor_readings_hourly FROM analytics_readonly;
REVOKE USAGE ON SCHEMA public FROM analytics_readonly;

DO $$
BEGIN
    EXECUTE format('REVOKE CONNECT ON DATABASE %I FROM analytics_readonly', current_database());
END $$;

DROP ROLE IF EXISTS analytics_readonly;

DROP MATERIALIZED VIEW IF EXISTS sensor_readings_hourly;

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
