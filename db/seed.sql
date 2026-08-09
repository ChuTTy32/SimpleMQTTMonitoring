-- Тестовые данные для разработки. НЕ миграция и НЕ в db/migrations/ — попадать в прод
-- при `migrate up` этот файл не должен. Применяется вручную: `make seed`.
--
-- Данные согласованы с реальным симулятором (sensor/simulator.py): топик
-- gateway/{GATEWAY_ID}/{metric}, GATEWAY_ID по умолчанию esp32-01, метрики
-- temperature/humidity/pressure. Без этих строк Consumer обязан дропать входящие
-- MQTT-сообщения — см. «Решение о провижининге устройств» в docs/PROJECT.md.
--
-- Идемпотентно: ON CONFLICT DO NOTHING по UNIQUE-колонкам (controllers.mqtt_gateway_id,
-- sensors.topic), повторный `make seed` не падает и не плодит дубли.
--
-- Внимание: точки с запятой внутри комментариев и строк здесь безопасны — файл
-- выполняется через psql, а не через golang-migrate с наивным разбиением
-- (см. db/migrations/README.md).

INSERT INTO controllers (name, mqtt_gateway_id, ip_address, location, is_active) VALUES
    ('Цех №1 — главный шлюз', 'esp32-01', '192.168.1.50', 'Цех №1, щит A', true),
    ('Цех №2 — резервный шлюз', 'esp32-02', '192.168.1.51', 'Цех №2, щит B', false)
ON CONFLICT (mqtt_gateway_id) DO NOTHING;

-- Датчики esp32-01 — те самые три метрики, что публикует симулятор.
-- Пороги заданы так, чтобы штатные значения симулятора были внутри диапазона,
-- а выбросы срабатывали — иначе алертинг нечем проверять.
INSERT INTO sensors (controller_id, name, topic, metric_type, unit, min_threshold, max_threshold, is_active)
SELECT c.id, s.name, s.topic, s.metric_type, s.unit, s.min_threshold, s.max_threshold, s.is_active
FROM controllers c
JOIN (VALUES
    ('esp32-01', 'Температура — цех №1', 'gateway/esp32-01/temperature', 'temperature', '°C',  15.0, 30.0, true),
    ('esp32-01', 'Влажность — цех №1',   'gateway/esp32-01/humidity',    'humidity',    '%',   30.0, 70.0, true),
    ('esp32-01', 'Давление — цех №1',    'gateway/esp32-01/pressure',    'pressure',    'Bar',  0.9,  1.2, true),
    ('esp32-02', 'Температура — цех №2', 'gateway/esp32-02/temperature', 'temperature', '°C',  15.0, 30.0, true),
    ('esp32-02', 'Влажность — цех №2',   'gateway/esp32-02/humidity',    'humidity',    '%',   30.0, 70.0, false)
) AS s(gateway_id, name, topic, metric_type, unit, min_threshold, max_threshold, is_active)
  ON s.gateway_id = c.mqtt_gateway_id
ON CONFLICT (topic) DO NOTHING;

INSERT INTO system_settings (key, value, description) VALUES
    ('dashboard_refresh_sec', '5',  'Интервал обновления дашборда в секундах'),
    ('alert_retention_days',  '90', 'Сколько дней хранить историю сработавших алертов')
ON CONFLICT (key) DO NOTHING;
