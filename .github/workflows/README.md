# GitHub Actions Workflows

Este diretório contém os workflows de CI/CD do projeto Edge Video.

## 📋 Workflows Disponíveis

### 1. `go-test.yml` - Testes e Qualidade de Código
**Trigger:** Push ou Pull Request em qualquer branch

**Jobs:**
- **test**: Executa testes unitários com cobertura
- **lint**: Verifica formatação e executa golangci-lint
- **build**: Verifica se o projeto compila
- **summary**: Resumo geral de todos os checks

**Badges sugeridos para README.md:**
```markdown
![Go Tests](https://github.com/T3-Labs/edge-video/actions/workflows/go-test.yml/badge.svg)
```

### 2. `build-and-push.yml` - Build e Deploy Docker
**Trigger:** 
- Criação de Release (tag)
- Manual via workflow_dispatch

**Ações:**
- Faz build da imagem Docker
- Push para GitHub Container Registry (ghcr.io)
- Cria tags: `versão` + `latest`

**Exemplo de uso:**
```bash
# Criar e publicar release
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# Usar a imagem
docker pull ghcr.io/t3-labs/edge-video:latest
docker pull ghcr.io/t3-labs/edge-video:1.0.0
```

## 🔧 Configuração Local

### Executar testes localmente:
```bash
# Testes unitários
go test -v -race -coverprofile=coverage.out ./...

# Ver cobertura
go tool cover -func=coverage.out
go tool cover -html=coverage.out

# Lint
golangci-lint run

# Formatação
gofmt -s -w .
```

### Instalar golangci-lint:
```bash
# Linux/macOS
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Ou via Go
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

## 📊 Cobertura de Testes

A cobertura mínima está configurada em **0%** (ajustável no workflow).

Para aumentar a cobertura mínima exigida, edite `go-test.yml`:
```yaml
MIN_COVERAGE=70  # 70% de cobertura mínima
```

## 🚀 Boas Práticas

1. **Sempre execute os testes localmente** antes de fazer push
2. **PRs devem passar em todos os checks** antes de merge
3. **Mantenha a cobertura de testes alta**
4. **Use commits semânticos** para facilitar changelogs automáticos
5. **Crie releases versionadas** seguindo Semantic Versioning (semver.org)

## 🔒 Secrets Necessários

### Para `build-and-push.yml`:
- `GITHUB_TOKEN` - Fornecido automaticamente pelo GitHub Actions

### Para deploy em produção (futuro):
Adicione em **Settings → Secrets and variables → Actions**:
- Credenciais de cloud providers
- Tokens de acesso a registries privados
- Variáveis de ambiente sensíveis

## 📚 Recursos

- [GitHub Actions Documentation](https://docs.github.com/actions)
- [golangci-lint Configuration](https://golangci-lint.run/usage/configuration/)
- [Docker Build Push Action](https://github.com/docker/build-push-action)
