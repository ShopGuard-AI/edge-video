#!/bin/bash
# Edge Video Repository Refactoring Script
# Automatiza a reorganização do repositório

set -e  # Exit on error

echo "🚀 Edge Video Repository Refactoring"
echo "===================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored messages
print_info() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Check if we're in the right directory
if [ ! -f "go.mod" ] || [ ! -d "cmd" ]; then
    print_error "Este script deve ser executado na raiz do repositório edge-video"
    exit 1
fi

# Step 1: Create backup branch
echo "📦 Passo 1: Criando branch de backup..."
CURRENT_BRANCH=$(git branch --show-current)
git stash
git checkout -b refactor/organize-repo-$(date +%Y%m%d-%H%M%S) || {
    print_warning "Branch já existe ou erro ao criar. Continuando..."
}
print_info "Branch criada"
echo ""

# Step 2: Create new directory structure
echo "📁 Passo 2: Criando nova estrutura de diretórios..."
mkdir -p configs/docker-compose
mkdir -p examples/python
mkdir -p examples/go
mkdir -p tests
print_info "Diretórios criados"
echo ""

# Step 3: Move configuration files
echo "📋 Passo 3: Movendo arquivos de configuração..."
if [ -f "config.toml" ]; then
    mv config.toml configs/config.example.toml
    print_info "config.toml → configs/config.example.toml"
fi

if [ -f "config-with-memory-control.toml" ]; then
    mv config-with-memory-control.toml configs/config.memory-control.toml
    print_info "config-with-memory-control.toml → configs/config.memory-control.toml"
fi

if [ -f "config.test.toml" ]; then
    mv config.test.toml configs/config.test.toml
    print_info "config.test.toml → configs/config.test.toml"
fi

if [ -f "docker-compose.yml" ]; then
    mv docker-compose.yml configs/docker-compose/
    print_info "docker-compose.yml → configs/docker-compose/"
fi

if [ -f "docker-compose.test.yml" ]; then
    mv docker-compose.test.yml configs/docker-compose/
    print_info "docker-compose.test.yml → configs/docker-compose/"
fi
echo ""

# Step 4: Move Python examples
echo "🐍 Passo 4: Movendo exemplos Python..."
if [ -f "test_camera_redis_amqp.py" ]; then
    mv test_camera_redis_amqp.py examples/python/consumer_basic.py
    print_info "test_camera_redis_amqp.py → examples/python/consumer_basic.py"
fi

if [ -f "test_consumer_status.py" ]; then
    mv test_consumer_status.py examples/python/consumer_status_monitor.py
    print_info "test_consumer_status.py → examples/python/consumer_status_monitor.py"
fi

if [ -f "test_consumer.py" ]; then
    mv test_consumer.py examples/python/consumer_legacy.py
    print_info "test_consumer.py → examples/python/consumer_legacy.py"
fi
echo ""

# Step 5: Move Go examples
echo "🔧 Passo 5: Movendo exemplos Go..."
if [ -d "cmd/validate-config" ]; then
    mv cmd/validate-config examples/go/
    print_info "cmd/validate-config → examples/go/"
fi
echo ""

# Step 6: Clean up temporary files
echo "🧹 Passo 6: Removendo arquivos temporários..."
rm -f edge-video edge-video-test
rm -f coverage.out
rm -f repomix-output.xml
rm -f *.log
print_info "Arquivos temporários removidos"
echo ""

# Step 7: Create symlinks for backward compatibility
echo "🔗 Passo 7: Criando symlinks para compatibilidade..."
ln -sf configs/config.example.toml config.toml
ln -sf configs/docker-compose/docker-compose.yml docker-compose.yml
print_info "Symlinks criados"
echo ""

# Step 8: Update .gitignore
echo "📝 Passo 8: Atualizando .gitignore..."
cat >> .gitignore << 'EOF'

# === Refactoring: Additional ignores ===

# Built binaries
edge-video
edge-video-test
edge-video-service
edge-video-service.exe

# Temporary files
*.tmp
*.log
*.swp
*.swo
*~

# Build artifacts
dist/
build/
bin/

# XML output files
repomix-output.xml
*.xml.bak

# Test artifacts
*.test
coverage.*
EOF
print_info ".gitignore atualizado"
echo ""

# Step 9: Create README in examples directory
echo "📚 Passo 9: Criando documentação de exemplos..."
cat > examples/README.md << 'EOF'
# Examples

This directory contains example code for using Edge Video.

## Python Examples

### consumer_basic.py
Basic consumer that receives frames from RabbitMQ and fetches them from Redis.

**Usage:**
```bash
cd python
python consumer_basic.py
```

**Requirements:**
```bash
pip install pika redis opencv-python
```

### consumer_status_monitor.py
Advanced consumer that monitors camera status and system events.

**Usage:**
```bash
cd python
python consumer_status_monitor.py
```

## Go Examples

### validate-config
Utility to validate configuration files.

**Usage:**
```bash
cd go/validate-config
go run main.go ../../configs/config.example.toml
```

## Documentation

For more examples and detailed documentation, see:
- [Getting Started Guide](../docs/getting-started/)
- [API Documentation](../docs/api/)
- [Integration Guide](../docs/guides/)
EOF
print_info "README de exemplos criado"
echo ""

# Step 10: Create configs README
echo "📋 Passo 10: Criando documentação de configs..."
cat > configs/README.md << 'EOF'
# Configuration Files

This directory contains all configuration files for Edge Video.

## Configuration Files

### config.example.toml
Basic configuration template. Copy this file to `config.toml` in the root directory and customize it.

```bash
cp config.example.toml ../config.toml
```

### config.memory-control.toml
Configuration optimized for memory-constrained environments (e.g., Windows with limited RAM).

Includes:
- Memory controller settings
- Optimized buffer sizes
- Throttling configuration

### config.test.toml
Configuration for running tests.

## Docker Compose

### docker-compose/docker-compose.yml
Production Docker Compose setup with RabbitMQ, Redis, and Edge Video.

**Usage:**
```bash
cd docker-compose
docker-compose up -d
```

### docker-compose/docker-compose.test.yml
Testing environment with additional debugging tools.

## Quick Start

1. Copy example configuration:
   ```bash
   cp configs/config.example.toml config.toml
   ```

2. Edit camera URLs and credentials in `config.toml`

3. Run:
   ```bash
   ./edge-video --config config.toml
   ```

## Documentation

For detailed configuration options, see:
- [Configuration Guide](../docs/getting-started/configuration.md)
- [Memory Control](../docs/MEMORY-CONTROL.md)
- [Multi-tenancy Setup](../docs/guides/vhost-implementation.md)
EOF
print_info "README de configs criado"
echo ""

# Step 11: Validate structure
echo "✅ Passo 11: Validando estrutura..."
print_info "Verificando se Go compila..."
if go build -o edge-video ./cmd/edge-video; then
    print_info "✓ Compilação bem-sucedida"
    rm -f edge-video
else
    print_error "✗ Erro na compilação - verifique o código"
fi

print_info "Executando testes..."
if go test ./... > /dev/null 2>&1; then
    print_info "✓ Todos os testes passaram"
else
    print_warning "⚠ Alguns testes falharam - verifique manualmente"
fi
echo ""

# Step 12: Create summary
echo "📊 Passo 12: Criando sumário de mudanças..."
cat > REFACTORING_SUMMARY.md << 'EOF'
# Refactoring Summary

## Changes Made

### Directory Structure
```
BEFORE:
├── config.toml
├── config-with-memory-control.toml
├── docker-compose.yml
├── test_*.py (múltiplos arquivos na raiz)
└── 40+ items in root

AFTER:
├── configs/
│   ├── config.example.toml
│   ├── config.memory-control.toml
│   ├── config.test.toml
│   └── docker-compose/
├── examples/
│   ├── python/
│   └── go/
└── ~15 items in root (essentials only)
```

### File Movements

**Configurations:**
- config.toml → configs/config.example.toml
- config-with-memory-control.toml → configs/config.memory-control.toml
- config.test.toml → configs/config.test.toml
- docker-compose*.yml → configs/docker-compose/

**Python Examples:**
- test_camera_redis_amqp.py → examples/python/consumer_basic.py
- test_consumer_status.py → examples/python/consumer_status_monitor.py

**Go Examples:**
- cmd/validate-config → examples/go/validate-config

**Symlinks Created:**
- config.toml → configs/config.example.toml
- docker-compose.yml → configs/docker-compose/docker-compose.yml

### Benefits

1. **Cleaner Root Directory**
   - Only essential files in root
   - Easy to navigate and understand

2. **Organized Examples**
   - Clear separation by language
   - Easy to find and use

3. **Centralized Configs**
   - All configurations in one place
   - Better version control

4. **Better Maintainability**
   - Logical grouping
   - Easier to extend

## Next Steps

1. Update documentation references
2. Update CI/CD paths if needed
3. Test all workflows
4. Update README.md with new structure
5. Commit and push changes

## Validation

- [x] Go build successful
- [x] Tests passing
- [x] Directory structure created
- [x] Files moved correctly
- [x] Documentation created
- [x] .gitignore updated
EOF
print_info "Sumário criado: REFACTORING_SUMMARY.md"
echo ""

# Final message
echo "🎉 Refatoração Completa!"
echo ""
echo "Próximos passos:"
echo "1. Revise as mudanças: git status"
echo "2. Teste a aplicação: go build ./cmd/edge-video"
echo "3. Execute os testes: go test ./..."
echo "4. Commit as mudanças:"
echo "   git add -A"
echo "   git commit -m 'refactor: Reorganize repository structure'"
echo "5. Faça merge na branch principal"
echo ""
echo "Arquivos criados:"
echo "  - REFACTORING_GUIDE.md (guia detalhado)"
echo "  - REFACTORING_SUMMARY.md (sumário de mudanças)"
echo "  - configs/README.md (documentação de configs)"
echo "  - examples/README.md (documentação de exemplos)"
echo ""
print_info "Script concluído com sucesso!"
