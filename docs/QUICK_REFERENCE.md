# 🚀 Quick Reference: Towncrier & Pre-commit

## Instalação Rápida

```bash
pip install pre-commit towncrier commitizen
pre-commit install
pre-commit install --hook-type commit-msg
```

## Comandos Diários

### Criar Changelog Fragment
```bash
./scripts/new-changelog.sh <tipo> "mensagem"
```

**Tipos:** `feature`, `bugfix`, `docs`, `removal`, `security`, `performance`, `refactor`, `misc`

### Commit
```bash
git add .
git commit -m "feat: sua mensagem"  # Hooks executam automaticamente
```

### Listar Fragments
```bash
./scripts/new-changelog.sh --list
```

## Release

### Preview
```bash
./scripts/build-changelog.sh --draft 1.0.0
```

### Gerar
```bash
./scripts/build-changelog.sh 1.0.0
git add CHANGELOG.md changelog.d/
git commit -m "chore: release v1.0.0"
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin main --tags
```

## Atalhos

### Executar Hooks Manualmente
```bash
pre-commit run --all-files
```

### Bypass Hooks (emergência)
```bash
git commit --no-verify -m "mensagem"
```

## Formato de Commit

Use [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` - nova funcionalidade
- `fix:` - correção de bug
- `docs:` - documentação
- `refactor:` - refatoração
- `test:` - testes
- `chore:` - manutenção
- `perf:` - performance
- `style:` - formatação

## Documentação Completa

- 📖 [docs/PRECOMMIT_TOWNCRIER_GUIDE.md](PRECOMMIT_TOWNCRIER_GUIDE.md)
- 📖 [docs/TOWNCRIER_SETUP.md](TOWNCRIER_SETUP.md)
- 📖 [changelog.d/README.md](../changelog.d/README.md)
