# Guia de Configuração: Pre-commit + Towncrier

Este guia mostra como configurar e usar o sistema de changelog automático com Towncrier e pre-commit hooks.

## 📦 Instalação

### 1. Instalar Pre-commit

```bash
# Via pip
pip install pre-commit

# Ou via pipx (recomendado)
pipx install pre-commit

# Verificar instalação
pre-commit --version
```

### 2. Instalar Towncrier

```bash
# Via pip
pip install towncrier

# Ou adicionar ao requirements
echo "towncrier>=23.11.0" >> requirements-dev.txt
pip install -r requirements-dev.txt
```

### 3. Instalar os Hooks

```bash
# Instalar hooks do pre-commit no repositório
pre-commit install

# Instalar hook para mensagens de commit (commitizen)
pre-commit install --hook-type commit-msg

# Verificar instalação
pre-commit --version
```

## 🚀 Uso Diário

### Workflow Completo:

#### 1. **Fazer Mudanças no Código**
```bash
git checkout -b feature/nova-funcionalidade
# ... fazer mudanças ...
```

#### 2. **Criar Fragment de Changelog**
```bash
# Sintaxe: <numero>.tipo.md
# Tipos: feature, bugfix, docs, removal, security, performance, refactor, misc

# Exemplo 1: Nova funcionalidade
echo "Adiciona suporte a PostgreSQL" > changelog.d/$(date +%s).feature.md

# Exemplo 2: Correção de bug
echo "Corrige memory leak no processamento de frames" > changelog.d/$(date +%s).bugfix.md

# Exemplo 3: Com número de issue
echo "Implementa retry automático para falhas de rede" > changelog.d/123.feature.md
```

#### 3. **Fazer Commit**
```bash
git add .
git commit -m "feat: adiciona suporte a PostgreSQL"
```

**O que acontece automaticamente:**
- ✅ Código Go é formatado (gofmt)
- ✅ Imports são organizados (goimports)
- ✅ `go mod tidy` é executado
- ✅ Lint é executado (go vet)
- ✅ Verifica se há changelog fragment (towncrier-check)
- ✅ Valida formato do commit (commitizen)
- ✅ Detecta segredos no código
- ✅ Valida arquivos YAML/TOML/JSON

#### 4. **Push para Remote**
```bash
git push origin feature/nova-funcionalidade
```

## 📝 Gerando o CHANGELOG

### Quando Criar Release:

```bash
# 1. Merge todas as features para main
git checkout main
git merge develop

# 2. Gerar CHANGELOG para nova versão
towncrier build --version 1.0.0

# Isso irá:
# - Coletar todos os fragments de changelog.d/
# - Gerar as notas de release no CHANGELOG.md
# - Remover os fragments processados

# 3. Commit e tag
git add CHANGELOG.md
git commit -m "chore: release v1.0.0"
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin main --tags
```

### Preview do CHANGELOG (Dry Run):

```bash
# Ver como ficará o changelog sem modificar arquivos
towncrier build --version 1.0.0 --draft
```

## 🎯 Tipos de Changelog Fragments

| Tipo | Emoji | Descrição | Exemplo |
|------|-------|-----------|---------|
| `feature` | ✨ | Nova funcionalidade | Adiciona cache Redis |
| `bugfix` | 🐛 | Correção de bug | Corrige race condition |
| `docs` | 📚 | Documentação | Atualiza README com exemplos |
| `removal` | 🗑️ | Remoção/depreciação | Remove API v1 depreciada |
| `security` | 🔒 | Correção de segurança | Adiciona validação de entrada |
| `performance` | ⚡ | Melhoria de performance | Otimiza query de banco de dados |
| `refactor` | ♻️ | Refatoração | Reestrutura módulo de cache |
| `misc` | 🔧 | Outras mudanças | Atualiza dependências |

## 🔧 Comandos Úteis

### Pre-commit:

```bash
# Executar todos os hooks manualmente
pre-commit run --all-files

# Executar hook específico
pre-commit run go-fmt --all-files
pre-commit run towncrier-check --all-files

# Atualizar versões dos hooks
pre-commit autoupdate

# Desinstalar hooks
pre-commit uninstall

# Bypass hooks temporariamente
git commit --no-verify -m "commit sem hooks"
```

### Towncrier:

```bash
# Listar fragments pendentes
ls -la changelog.d/*.md

# Validar configuração
towncrier --help

# Gerar changelog sem remover fragments
towncrier build --version 1.0.0 --keep

# Gerar changelog automaticamente
towncrier build --version 1.0.0 --yes
```

## 🛠️ Troubleshooting

### Erro: "towncrier-check failed"

**Problema:** Você tentou fazer commit sem criar um fragment de changelog.

**Solução:**
```bash
# Opção 1: Criar fragment
echo "Sua mudança aqui" > changelog.d/$(date +%s).feature.md
git add changelog.d/
git commit -m "feat: sua mudança"

# Opção 2: Bypass (não recomendado)
git commit --no-verify -m "feat: sua mudança"
```

### Erro: "go-fmt failed"

**Problema:** Código não está formatado corretamente.

**Solução:**
```bash
# Pre-commit já formatou automaticamente
git add -u
git commit -m "feat: sua mudança"
```

### Erro: "commitizen failed"

**Problema:** Mensagem de commit não segue o formato Conventional Commits.

**Solução:**
Use o formato: `tipo: descrição`

Tipos válidos:
- `feat:` - nova funcionalidade
- `fix:` - correção de bug
- `docs:` - mudanças na documentação
- `refactor:` - refatoração de código
- `test:` - adiciona ou corrige testes
- `chore:` - mudanças em build, CI, etc.
- `perf:` - melhoria de performance
- `style:` - mudanças de formatação

**Exemplo:**
```bash
git commit -m "feat: adiciona suporte a PostgreSQL"
```

### Erro: "detect-secrets failed"

**Problema:** Possível segredo detectado no código.

**Solução:**
```bash
# Revisar o arquivo apontado
# Se for falso positivo, atualizar baseline:
detect-secrets scan --baseline .secrets.baseline

# E commitar
git add .secrets.baseline
```

## 🎨 Customização

### Modificar Tipos de Fragments:

Edite `pyproject.toml`:

```toml
[[tool.towncrier.type]]
directory = "breaking"
name = "💥 Breaking Changes"
showcontent = true
```

### Modificar Hooks do Pre-commit:

Edite `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: https://github.com/seu/hook
    rev: v1.0.0
    hooks:
      - id: seu-hook
```

## 📚 Recursos

- [Pre-commit Documentation](https://pre-commit.com/)
- [Towncrier Documentation](https://towncrier.readthedocs.io/)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [Keep a Changelog](https://keepachangelog.com/)
- [Semantic Versioning](https://semver.org/)

## 🤝 Contribuindo

Ao contribuir com este projeto:

1. ✅ **Sempre crie um fragment de changelog** para suas mudanças
2. ✅ **Use commits semânticos** (feat:, fix:, docs:, etc.)
3. ✅ **Deixe os hooks executarem** (não use --no-verify sem necessidade)
4. ✅ **Revise o preview do changelog** antes de criar release

---

**Última atualização:** 2025-11-06
