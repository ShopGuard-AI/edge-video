This directory contains changelog fragments for Towncrier.

## Como Usar

### 1. Criar um Fragment de Changelog

Quando você faz uma mudança, crie um arquivo neste diretório com o formato:

```
<número-da-issue>.<tipo>.md
```

**Exemplo:**
```bash
# Para issue #123, feature nova
echo "Adiciona armazenamento Redis para frames" > changelog.d/123.feature.md

# Para issue #456, bugfix
echo "Corrige erro de conexão com RabbitMQ" > changelog.d/456.bugfix.md

# Sem issue number, use um identificador único
echo "Atualiza documentação do README" > changelog.d/$(date +%s).docs.md
```

### 2. Tipos de Fragments Disponíveis

- **feature** - ✨ Nova funcionalidade
- **bugfix** - 🐛 Correção de bug
- **docs** - 📚 Mudanças na documentação
- **removal** - 🗑️ Remoções e depreciações
- **security** - 🔒 Correções de segurança
- **performance** - ⚡ Melhorias de performance
- **refactor** - ♻️ Refatoração de código
- **misc** - 🔧 Outras mudanças

### 3. Gerar o CHANGELOG

```bash
# Gerar changelog para uma nova versão
towncrier build --version 1.0.0

# Preview sem modificar arquivos
towncrier build --version 1.0.0 --draft

# Gerar e fazer commit automaticamente
towncrier build --version 1.0.0 --yes
```

### 4. Exemplo de Fragment

**changelog.d/123.feature.md:**
```markdown
Adiciona suporte a autenticação Redis com senha configurável via config.toml
```

**changelog.d/456.bugfix.md:**
```markdown
Corrige race condition na captura de frames de múltiplas câmeras
```

### 5. Ignorar o Hook do Pre-commit

Se precisar fazer um commit sem fragment (ex: commits em main):

```bash
git commit --no-verify -m "chore: atualiza dependências"
```

## Estrutura de Arquivo Fragment

Cada fragment é um arquivo simples de texto markdown contendo:
- Uma linha descrevendo a mudança
- Opcionalmente, mais detalhes em parágrafos adicionais

## Integração com CI/CD

O pre-commit hook `towncrier-check` valida que:
- ✅ Branches de feature têm pelo menos um fragment
- ✅ Os fragments seguem o formato correto
- ✅ Não há fragments duplicados

## Comandos Úteis

```bash
# Instalar towncrier
pip install towncrier

# Verificar configuração
towncrier --help

# Listar fragments pendentes
ls -la changelog.d/*.md

# Limpar fragments após build
# (towncrier faz isso automaticamente com --yes)
```

## Recursos

- [Towncrier Docs](https://towncrier.readthedocs.io/)
- [Keep a Changelog](https://keepachangelog.com/)
- [Semantic Versioning](https://semver.org/)
