#!/bin/bash

# Script para configurar ambiente de desenvolvimento

set -e

echo "🚀 Configurando ambiente de desenvolvimento Edge Video..."

# Verifica se uv está instalado
if ! command -v uv &> /dev/null; then
    echo "❌ uv não encontrado. Instale o uv primeiro:"
    echo "curl -LsSf https://astral.sh/uv/install.sh | sh"
    exit 1
fi

# Sincroniza dependências
echo "📦 Instalando dependências..."
uv sync --dev

# Executa testes
echo "🧪 Executando testes..."
uv run pytest --cov=src --cov-report=term-missing

# Executa linting
echo "🔍 Executando linting..."
uv run ruff check src/
uv run ruff format --check src/

echo "✅ Ambiente configurado com sucesso!"
echo ""
echo "📝 Comandos úteis:"
echo "  uv run pytest               # Executar testes"
echo "  uv run pytest --cov=src     # Testes com cobertura"
echo "  uv run ruff check src/       # Verificar código"
echo "  uv run ruff format src/      # Formatar código"
echo "  uv run python main_refactored.py  # Executar aplicação"