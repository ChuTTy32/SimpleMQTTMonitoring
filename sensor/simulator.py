import os
import time
import json
import random
import math
import paho.mqtt.client as mqtt
from dotenv import load_dotenv

load_dotenv()

BROKER_HOST = str(os.getenv('MQTT_BROKER', "huy"))
BROKER_PORT = int(os.getenv('MQTT_BROKER_PORT', 666))
GATEWAY_ID = os.getenv("GATEWAY_ID", "esp32-01")
PUBLISH_INTERVAL_SEC = float(os.getenv("PUBLISH_INTERVAL_SEC", 2))


def generate_temperature(t: float) -> float:
    return 23 + 3 * math.sin(t / 30) + random.uniform(-0.3, 0.3)


def generate_humidity(t: float) -> float:
    val = 60 + 10 * math.sin(t / 45) + random.uniform(-2, 2)
    return max(0, min(100, val))  # клип в физический диапазон 0-100%


def generate_pressure(t: float) -> float:
    return 1013 + 2 * math.sin(t / 90) + random.uniform(-0.5, 0.5)


# Добавлены аргументы, чтобы код не ломался на разных версиях paho-mqtt
def on_connect(client, userdata, flags, rc, properties=None):
    if rc == 0:
        print(f"[{GATEWAY_ID}] Connected to broker at {BROKER_HOST}:{BROKER_PORT}")
    else:
        print(f"[{GATEWAY_ID}] Connection failed, code={rc}")


def on_disconnect(client, userdata, rc, properties=None):
    print(f"[{GATEWAY_ID}] Disconnected, code={rc}")


def build_payload(value: float) -> str:
    return json.dumps({
        "value": round(value, 2),
        "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    })


def main():
    # Безопасная инициализация клиента для paho-mqtt v1.x и v2.x
    try:
        client = mqtt.Client(client_id=GATEWAY_ID, callback_api_version=mqtt.CallbackAPIVersion.VERSION2)
    except AttributeError:
        client = mqtt.Client(client_id=GATEWAY_ID)

    client.on_connect = on_connect
    client.on_disconnect = on_disconnect

    # ЗАЩИТА ОТ ГОНКИ ЗАПУСКА: цикл ожидания готовности брокера
    connected = False
    while not connected:
        try:
            print(f"[{GATEWAY_ID}] Trying to connect to broker at {BROKER_HOST}:{BROKER_PORT}...")
            client.connect(BROKER_HOST, BROKER_PORT, keepalive=60)
            connected = True
        except Exception as e:
            print(f"[{GATEWAY_ID}] Broker is not ready yet ({e}). Retrying in 2 seconds...")
            time.sleep(2)

    client.loop_start()

    start_time = time.time()
    try:
        while True:
            elapsed = time.time() - start_time

            readings = {
                "temperature": generate_temperature(elapsed),
                "humidity": generate_humidity(elapsed),
                "pressure": generate_pressure(elapsed),
            }

            for sensor_type, value in readings.items():
                topic = f"gateway/{GATEWAY_ID}/{sensor_type}"
                payload = build_payload(value)
                client.publish(topic, payload, qos=0)
                print(f"[{GATEWAY_ID}] {topic} -> {payload}")

            time.sleep(PUBLISH_INTERVAL_SEC)
    except KeyboardInterrupt:
        print(f"[{GATEWAY_ID}] Shutting down...")
    finally:
        client.loop_stop()
        client.disconnect()


if __name__ == "__main__":
    main()