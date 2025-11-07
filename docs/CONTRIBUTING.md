# Guia de Contribuição

Obrigado por considerar contribuir para o Edge Video! Este documento fornece diretrizes para contribuir com o projeto.

## 📋 Índice

- [Código de Conduta](#código-de-conduta)
- [Como Contribuir](#como-contribuir)
- [Padrões de Commit](#padrões-de-commit)
- [Processo de Review](#processo-de-review)
- [Configuração do Ambiente](#configuração-do-ambiente)

## 🤝 Código de Conduta

Este projeto segue um código de conduta que todos os colaboradores devem respeitar. Seja respeitoso, inclusivo e construtivo em suas interações.

## 🚀 Como Contribuir

### 1. Fork e Clone

```bash
# Fork o repositório no GitHub
# Clone seu fork
git clone https://github.com/SEU_USUARIO/edge-video.git
cd edge-video

# Adicione o repositório upstream
git remote add upstream https://github.com/T3-Labs/edge-video.git
```

### 2. Crie uma Branch

```bash
# Sincronize com upstream
git checkout develop
git pull upstream develop

# Crie sua branch a partir de develop
git checkout -b feature/sua-feature
# ou
git checkout -b fix/seu-bugfix
```

### 3. Faça suas Alterações

- Siga os [padrões de código](development/contributing.md)
- Adicione testes para novas funcionalidades
- Atualize a documentação conforme necessário
- Execute os testes localmente

```bash
# Testes Go
go test ./...

# Linters
go vet ./...
golangci-lint run
```

### 4. Commit suas Mudanças

Use o sistema de [Pre-commit + Towncrier](development/precommit-towncrier.md):

```bash
# Adicione um changelog fragment
towncrier create 123.feature.md --content "Nova funcionalidade X"

# Commit (pre-commit rodará automaticamente)
git add .
git commit -m "feat: adiciona funcionalidade X"
```

### 5. Push e Pull Request

```bash
# Push para seu fork
git push origin feature/sua-feature

# Abra um Pull Request no GitHub
# Base: develop
# Compare: feature/sua-feature
```

## 📝 Padrões de Commit

Seguimos [Conventional Commits](https://www.conventionalcommits.org/):

```
<tipo>[escopo opcional]: <descrição>

[corpo opcional]

[rodapé opcional]
```

### Tipos de Commit

- `feat`: Nova funcionalidade
- `fix`: Correção de bug
- `docs`: Documentação
- `style`: Formatação (sem mudança de código)
- `refactor`: Refatoração
- `test`: Adição/correção de testes
- `chore`: Tarefas de manutenção
- `perf`: Melhoria de performance
- `ci`: Alterações em CI/CD

### Exemplos

```bash
# Feature
git commit -m "feat(camera): add support for H.265 codec"

# Bug fix
git commit -m "fix(redis): resolve connection timeout issue"

# Documentation
git commit -m "docs: update installation guide for Windows"

# Breaking change
git commit -m "feat!: change config file format to TOML

BREAKING CHANGE: Config files now use TOML instead of YAML.
See migration guide in docs/guides/migration.md"
```

## 🔍 Processo de Review

### Checklist do PR

Antes de abrir um PR, certifique-se de que:

- [ ] O código compila sem erros
- [ ] Todos os testes passam
- [ ] Novos testes foram adicionados (se aplicável)
- [ ] Documentação foi atualizada (se aplicável)
- [ ] Changelog fragment foi criado
- [ ] Pre-commit hooks passam
- [ ] Commits seguem Conventional Commits
- [ ] PR tem título descritivo
- [ ] PR inclui descrição detalhada

### O que Esperamos

1. **Código Limpo**: Siga as convenções do Go
2. **Testes**: Cobertura mínima de 80%
3. **Documentação**: Funções públicas documentadas
4. **Performance**: Não degrade significativamente
5. **Segurança**: Sem vulnerabilidades conhecidas

### Tempo de Review

- PRs pequenos: 1-2 dias úteis
- PRs médios: 2-4 dias úteis
- PRs grandes: Considere dividir em PRs menores

## 🛠️ Configuração do Ambiente

### Pré-requisitos

- Go 1.24+
- Docker & Docker Compose
- Git
- Make (opcional, mas recomendado)

### Setup Inicial

```bash
# 1. Instalar dependências
go mod download

# 2. Copiar arquivo de configuração
cp config.yaml.example config.yaml

# 3. Instalar pre-commit hooks
pip install pre-commit towncrier
pre-commit install

# 4. Subir dependências (Redis, RabbitMQ)
docker-compose up -d redis rabbitmq

# 5. Executar testes
go test ./...

# 6. Executar aplicação
go run cmd/edge-video/main.go
```

### Ferramentas Úteis

#### Linters

```bash
# Instalar golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Executar
golangci-lint run
```

#### Testes com Coverage

```bash
# Gerar relatório de cobertura
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Ver no browser
xdg-open coverage.html
```

#### Benchmark

```bash
# Executar benchmarks
go test -bench=. -benchmem ./...
```

## 📚 Recursos Adicionais

- [Documentação Completa](../index.md)
- [Guia de Desenvolvimento](development/contributing.md)
- [Pre-commit + Towncrier](development/precommit-towncrier.md)
- [Testes](development/testing.md)
- [CI/CD](development/cicd.md)

## ❓ Dúvidas?

- Abra uma [Issue](https://github.com/T3-Labs/edge-video/issues)
- Participe das [Discussions](https://github.com/T3-Labs/edge-video/discussions)
- Entre em contato: [T3 Labs](https://github.com/T3-Labs)

---

Obrigado por contribuir! 🎉
