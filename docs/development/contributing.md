# Contribuindo

Obrigado por considerar contribuir para o Edge Video!

## 📖 Guia Completo de Contribuição

Para informações detalhadas sobre como contribuir, incluindo:

- Padrões de código
- Processo de review  
- Configuração do ambiente
- Padrões de commit (Conventional Commits)
- Sistema de changelog (Towncrier)

Consulte o [**Guia de Contribuição Completo**](../CONTRIBUTING.md).

## 🚀 Início Rápido

### 1. Fork e Clone

```bash
git clone https://github.com/SEU_USUARIO/edge-video.git
cd edge-video
git remote add upstream https://github.com/T3-Labs/edge-video.git
```

### 2. Configurar Ambiente

```bash
# Instalar dependências Go
go mod download

# Instalar pre-commit
pip install pre-commit towncrier
pre-commit install

# Subir dependências
docker-compose up -d redis rabbitmq
```

### 3. Criar Branch e Desenvolver

```bash
git checkout -b feature/minha-feature
# Faça suas alterações...
go test ./...
```

### 4. Commit com Changelog

```bash
# Criar changelog fragment
towncrier create 123.feature.md --content "Descrição da mudança"

# Commit (pre-commit rodará automaticamente)
git commit -m "feat: adiciona nova funcionalidade"
```

### 5. Push e Pull Request

```bash
git push origin feature/minha-feature
# Abra PR no GitHub: develop ← feature/minha-feature
```

## 📝 Padrões de Commit

Seguimos [Conventional Commits](https://www.conventionalcommits.org/):

- `feat`: Nova funcionalidade
- `fix`: Correção de bug
- `docs`: Documentação
- `refactor`: Refatoração
- `test`: Testes
- `chore`: Manutenção

**Exemplo:**
```bash
git commit -m "feat(camera): add H.265 codec support"
```

## ✅ Checklist do PR

Antes de abrir um PR:

- [ ] Código compila sem erros
- [ ] Testes passam (`go test ./...`)
- [ ] Pre-commit hooks passam
- [ ] Changelog fragment criado
- [ ] Documentação atualizada
- [ ] Commits seguem Conventional Commits

## 🔗 Links Úteis

- [Pre-commit & Changelog](precommit-towncrier.md)
- [Testes](testing.md)
- [CI/CD](cicd.md)
- [Guia Completo](../CONTRIBUTING.md)

## 💬 Dúvidas?

- [Issues](https://github.com/T3-Labs/edge-video/issues)
- [Discussions](https://github.com/T3-Labs/edge-video/discussions)

---

Obrigado por contribuir! 🎉
