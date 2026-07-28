import os
import time
import json
import random
import math
import paho.mqtt.client as mqtt
from dotenv import load_dotenv

load_dotenv()

BROKER_HOST = os.getenv("MQTT_BROKER_HOST", "localhost")
BROKER_PORT = int(os.getenv("MQTT_BROKER_PORT", 1883))
GATEWAY_ID = os.getenv("GATEWAY_ID", "esp32-01")
PUBLISH_INTERVAL_SEC = float(os.getenv("PUBLISH_INTERVAL_SEC", 2))


def generate_temperature(t: float) -> float:
    return 23 + 3 * math.sin(t / 30) + random.uniform(-0.3, 0.3)


def generate_humidity(t: float) -> float:
    val = 60 + 10 * math.sin(t / 45) + random.uniform(-2, 2)
    return max(0, min(100, val))  # клип в физический диапазон 0-100%


def generate_pressure(t: float) -> float:
    return 1013 + 2 * math.sin(t / 90) + random.uniform(-0.5, 0.5)


def on_connect(client, userdata, flags, rc):
    if rc == 0:
        print(f"[{GATEWAY_ID}] Connected to broker at {BROKER_HOST}:{BROKER_PORT}")
    else:
        print(f"[{GATEWAY_ID}] Connection failed, code={rc}")


def on_disconnect(client, userdata, rc):
    print(f"[{GATEWAY_ID}] Disconnected, code={rc}")


def build_payload(value: float) -> str:
    return json.dumps({
        "value": round(value, 2),
        "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    })


def main():
    client = mqtt.Client(client_id=GATEWAY_ID)
    client.on_connect = on_connect
    client.on_disconnect = on_disconnect

    client.connect(BROKER_HOST, BROKER_PORT, keepalive=60)
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