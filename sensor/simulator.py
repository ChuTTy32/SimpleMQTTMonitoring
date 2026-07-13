import paho.mqtt.client as mqtt
import time
import random
import json
import os

# Берем адрес брокера из переменных окружения, по умолчанию mosquitto
BROKER = os.getenv("MQTT_BROKER", "mosquitto")
PORT = 1883
TOPIC = "sensors/temperature"

def on_connect(client, userdata, flags, rc):
    if rc == 0:
        print(f"Симулятор успешно подключен к {BROKER}")
    else:
        print(f"Ошибка подключения, код: {rc}")

client = mqtt.Client("python_simulator")
client.on_connect = on_connect

# Пытаемся подключиться в цикле, пока брокер не поднимется
while True:
    try:
        client.connect(BROKER, PORT, 60)
        break
    except ConnectionRefusedError:
        print("Брокер недоступен, ждем 2 секунды...")
        time.sleep(2)

client.loop_start()

if __name__ == "__main__":
    while True:
        # Генерируем температуру от 20.0 до 30.0 градусов
        temp = round(random.uniform(20.0, 30.0), 2)
        payload = json.dumps({"temperature": temp})
        
        client.publish(TOPIC, payload)
        print(f"Симулятор отправил: {payload}")
        
        time.sleep(5)
