FROM python:3.10-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY simulator.py .
COPY .env-example .env
# Отключаем буферизацию вывода, чтобы сразу видеть логи
ENV PYTHONUNBUFFERED=1

CMD ["python", "simulator.py"]
