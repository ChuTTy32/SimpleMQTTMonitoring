-include .env
export

COMPOSE := docker compose
# x-multi-statement=true — см. пояснение у сервиса migrate в docker-compose.yml:
# без него CREATE MATERIALIZED VIEW (continuous aggregate) падает в неявной транзакции.
DB_URL  := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@db:5432/$(POSTGRES_DB)?sslmode=disable&x-multi-statement=true

.DEFAULT_GOAL := help
.PHONY: help up up-build down down-v restart ps logs logs-one build \
        migrate-up migrate-down migrate-version seed db-shell

help: ## Показать список команд
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

up: ## Поднять все сервисы в фоне (перед этим проверь `make ps` — не запущено ли уже)
	$(COMPOSE) up -d

up-build: ## Поднять все сервисы, пересобрав образы
	$(COMPOSE) up -d --build

down: ## Остановить и удалить контейнеры (volume с данными БД сохраняется)
	$(COMPOSE) down

down-v: ## Остановить и удалить контейнеры ВМЕСТЕ С volume — ПОТЕРЯ ДАННЫХ БД
	$(COMPOSE) down -v

restart: down up ## Перезапустить всё (down + up)

ps: ## Статус контейнеров
	$(COMPOSE) ps

logs: ## Логи всех сервисов (Ctrl+C для выхода)
	$(COMPOSE) logs -f

logs-one: ## Логи одного сервиса: make logs-one SERVICE=db
	$(COMPOSE) logs -f $(SERVICE)

build: ## Пересобрать образы без запуска
	$(COMPOSE) build

migrate-up: ## Применить миграции вручную (обычно не нужно — сервис migrate делает это сам при up)
	$(COMPOSE) run --rm migrate -path=/migrations -database "$(DB_URL)" up

migrate-down: ## Откатить последнюю миграцию
	$(COMPOSE) run --rm migrate -path=/migrations -database "$(DB_URL)" down 1

migrate-version: ## Показать текущую версию миграции
	$(COMPOSE) run --rm migrate -path=/migrations -database "$(DB_URL)" version

seed: ## Залить тестовые данные (db/seed.sql) — контроллеры, датчики, настройки
	$(COMPOSE) exec -T db psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -v ON_ERROR_STOP=1 < db/seed.sql

db-shell: ## Зайти в psql внутри контейнера db
	$(COMPOSE) exec db psql -U $(POSTGRES_USER) -d $(POSTGRES_DB)
