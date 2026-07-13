FROM python:3.10-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY simulator.py .

# Отключаем буферизацию вывода, чтобы сразу видеть логи
ENV PYTHONUNBUFFERED=1

CMD ["python", "simulator.py"]
