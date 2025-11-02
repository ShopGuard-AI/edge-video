.PHONY: help install test lint format run clean dev-setup

help: ## Mostrar esta ajuda
	@echo "Comandos disponíveis:"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

install: ## Instalar dependências
	uv sync --dev

test: ## Executar testes
	uv run pytest

test-cov: ## Executar testes com cobertura
	uv run pytest --cov=src --cov-report=term-missing --cov-report=html

lint: ## Verificar código com ruff
	uv run ruff check src/ tests/

format: ## Formatar código com ruff
	uv run ruff format src/ tests/

format-check: ## Verificar formatação sem alterar
	uv run ruff format --check src/ tests/

run: ## Executar aplicação refatorada
	uv run python main_refactored.py

run-original: ## Executar aplicação original
	python test_consumer.py

clean: ## Limpar arquivos temporários
	find . -type f -name "*.pyc" -delete
	find . -type d -name "__pycache__" -delete
	find . -type d -name "*.egg-info" -exec rm -rf {} +
	rm -rf .pytest_cache/
	rm -rf htmlcov/
	rm -rf .coverage

dev-setup: ## Configurar ambiente de desenvolvimento
	./setup-dev.sh

docker-up: ## Subir containers Docker
	docker compose up -d

docker-down: ## Parar containers Docker
	docker compose down

docker-logs: ## Ver logs dos containers
	docker compose logs -f