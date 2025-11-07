# Guia de Desenvolvimento - CI/CD

## 🚀 Workflows Implementados

### 1. **Go Tests CI** (`go-test.yml`)
Executa automaticamente em **qualquer push ou PR** em **qualquer branch**.

#### Jobs:
- ✅ **test**: Testes unitários com race detector e cobertura
- ✅ **lint**: Análise de código com golangci-lint
- ✅ **build**: Verificação de compilação
- ✅ **summary**: Resumo geral dos resultados

### 2. **Docker Build & Push** (`build-and-push.yml`)
Executa apenas quando você **cria uma release tag**.

#### Ações:
- Build da imagem Docker
- Push para GitHub Container Registry
- Tags: `versão` + `latest`

---

## 📝 Workflow de Desenvolvimento

### Passo a Passo:

#### 1. **Desenvolvimento Local**
```bash
# Clone o repositório
git clone https://github.com/T3-Labs/edge-video.git
cd edge-video

# Instale as dependências
go mod download

# Execute os testes
go test -v -race ./...

# Verifique a cobertura
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Execute o lint
golangci-lint run

# Formate o código
gofmt -s -w .
```

#### 2. **Criar Feature Branch**
```bash
git checkout -b feature/nova-funcionalidade
```

#### 3. **Fazer Commits**
```bash
git add .
git commit -m "feat: adiciona nova funcionalidade"
```

#### 4. **Push e Criar PR**
```bash
git push origin feature/nova-funcionalidade
```
- **Automático**: Os testes serão executados no GitHub Actions
- Aguarde todos os checks passarem antes de fazer merge

#### 5. **Merge para Main**
```bash
git checkout main
git pull origin main
git merge feature/nova-funcionalidade
git push origin main
```
- **Automático**: Os testes serão executados novamente

#### 6. **Criar Release**
```bash
# Criar tag local
git tag -a v1.0.0 -m "Release v1.0.0 - Descrição das mudanças"

# Push da tag
git push origin v1.0.0
```
- **Automático**: Build do Docker e push para GHCR

---

## 🛠️ Ferramentas Necessárias

### Instalação Local:

#### golangci-lint
```bash
# macOS/Linux
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Ou via Go
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

#### Go 1.24
```bash
# Baixe em: https://go.dev/dl/
```

---

## 📊 Cobertura de Testes

### Atual:
- **pkg/config**: 80.0% ✅
- **Outros pacotes**: 0.0% (adicionar testes)

### Meta:
- Aumentar cobertura para pelo menos **70%**

### Como melhorar:
1. Adicione testes em `pkg/camera/`
2. Adicione testes em `internal/storage/`
3. Adicione testes em `internal/metadata/`
4. Adicione testes em `pkg/mq/`

---

## 🔍 Verificações Locais Antes de Commit

Execute este checklist:

```bash
# 1. Testes passando?
go test ./...

# 2. Cobertura adequada?
go test -cover ./...

# 3. Lint sem erros?
golangci-lint run

# 4. Código formatado?
gofmt -l .

# 5. Build funciona?
go build ./cmd/edge-video
```

Se tudo passar ✅, faça o commit!

---

## 🐛 Troubleshooting

### Testes Falhando no CI mas Passando Localmente?
- Verifique se todas as dependências estão no `go.mod`
- Execute `go mod tidy`
- Verifique se não há arquivos locais não comitados

### golangci-lint Retornando Erros?
```bash
# Ver detalhes
golangci-lint run --verbose

# Corrigir automaticamente alguns problemas
golangci-lint run --fix
```

### Build Docker Falhando?
- Verifique se o `Dockerfile` está atualizado
- Teste localmente: `docker build -t edge-video:test .`

---

## 📚 Recursos Úteis

- [GitHub Actions Docs](https://docs.github.com/actions)
- [golangci-lint](https://golangci-lint.run/)
- [Go Testing](https://go.dev/doc/tutorial/add-a-test)
- [Semantic Versioning](https://semver.org/)

---

## 🎯 Boas Práticas

1. ✅ **Sempre execute testes localmente** antes de push
2. ✅ **Use commits semânticos**: `feat:`, `fix:`, `docs:`, `refactor:`
3. ✅ **Mantenha PRs pequenos** e focados
4. ✅ **Documente mudanças** no changelog da release
5. ✅ **Não force push** em branches compartilhadas
6. ✅ **Revise código** antes de aprovar PRs

---

## 🚀 Deploy em Produção

### Usar a imagem Docker:
```bash
# Latest
docker pull ghcr.io/t3-labs/edge-video:latest

# Versão específica
docker pull ghcr.io/t3-labs/edge-video:1.0.0

# Executar
docker run -d \
  --name edge-video \
  -v ./config.toml:/app/config.toml \
  ghcr.io/t3-labs/edge-video:latest
```

---

**Última atualização:** 2025-11-06
