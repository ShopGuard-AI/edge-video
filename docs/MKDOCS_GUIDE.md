# Guia de Configuração: MkDocs Documentation

Este guia explica como trabalhar com a documentação do Edge Video usando MkDocs.

## 📦 Instalação Local

### 1. Instalar Dependências

```bash
# Criar ambiente virtual (recomendado)
python3 -m venv .venv-docs
source .venv-docs/bin/activate

# Instalar dependências
pip install -r requirements-docs.txt
```

### 2. Servir Documentação Localmente

```bash
# Servir com hot-reload
mkdocs serve

# Acessar em: http://localhost:8000
```

### 3. Build da Documentação

```bash
# Build para produção
mkdocs build

# Arquivos gerados em: site/
```

## 📝 Estrutura da Documentação

```
docs/
├── index.md                 # Página inicial
├── getting-started/         # Guia de início
│   ├── installation.md
│   ├── configuration.md
│   └── quickstart.md
├── architecture/            # Arquitetura
│   ├── overview.md
│   ├── components.md
│   └── data-flow.md
├── features/                # Funcionalidades
│   ├── camera-capture.md
│   ├── redis-storage.md
│   ├── metadata.md
│   └── message-queue.md
├── guides/                  # Guias práticos
│   ├── docker.md
│   ├── advanced-config.md
│   ├── monitoring.md
│   └── troubleshooting.md
├── development/             # Desenvolvimento
│   ├── contributing.md
│   ├── precommit-towncrier.md
│   ├── testing.md
│   └── cicd.md
├── api/                     # API Reference
│   ├── config.md
│   ├── camera.md
│   ├── storage.md
│   └── mq.md
├── about/                   # Sobre
│   ├── license.md
│   └── credits.md
├── changelog.md             # Changelog
├── stylesheets/             # CSS customizado
│   └── extra.css
└── javascripts/             # JS customizado
    ├── extra.js
    └── mathjax.js
```

## ✍️ Escrevendo Documentação

### Sintaxe Básica

```markdown
# Título H1

## Título H2

Parágrafo com **negrito** e *itálico*.

- Lista item 1
- Lista item 2

1. Lista numerada
2. Item 2

[Link](https://exemplo.com)

![Imagem](path/to/image.png)

\```python
# Bloco de código
print("Hello World")
\```
```

### Admonitions

```markdown
!!! note "Nota"
    Conteúdo da nota

!!! tip "Dica"
    Dica útil

!!! warning "Aviso"
    Conteúdo de aviso

!!! danger "Perigo"
    Alerta importante
```

### Tabs

```markdown
=== "Tab 1"

    Conteúdo da tab 1

=== "Tab 2"

    Conteúdo da tab 2
```

### Diagramas Mermaid

```markdown
\```mermaid
graph LR
    A[Início] --> B[Processo]
    B --> C[Fim]
\```
```

### Grids

```markdown
<div class="grid cards" markdown>

-   :material-icon:{ .lg } **Título**
    
    Descrição

-   :material-icon:{ .lg } **Título 2**
    
    Descrição 2

</div>
```

## 🚀 Deploy

### GitHub Pages (Automático via CI)

O deploy é feito automaticamente pelo GitHub Actions quando você faz push para `main`.

**Workflow:** `.github/workflows/mkdocs.yml`

### Deploy Manual

```bash
# Build e deploy
mkdocs gh-deploy

# Ou especificar branch
mkdocs gh-deploy --force
```

## 🎨 Customização

### Adicionar Nova Página

1. Criar arquivo em `docs/`
2. Adicionar no `nav` em `mkdocs.yml`

```yaml
nav:
  - Home: index.md
  - Nova Seção:
      - Nova Página: nova-secao/pagina.md
```

### Modificar Tema

Editar `mkdocs.yml`:

```yaml
theme:
  palette:
    primary: indigo
    accent: blue
```

### Adicionar CSS Customizado

Editar `docs/stylesheets/extra.css`

### Adicionar JavaScript

Editar `docs/javascripts/extra.js`

## 📊 Plugins Disponíveis

| Plugin | Descrição |
|--------|-----------|
| `search` | Busca na documentação |
| `git-revision-date-localized` | Data de última modificação |
| `minify` | Minificação de HTML/CSS/JS |
| `awesome-pages` | Navegação automática |

## 🔧 Comandos Úteis

### Desenvolvimento

```bash
# Servir com hot-reload
mkdocs serve

# Servir em porta específica
mkdocs serve -a 0.0.0.0:8080

# Build
mkdocs build

# Build strict (falha em warnings)
mkdocs build --strict
```

### Validação

```bash
# Verificar links quebrados
mkdocs build --strict

# Validar configuração
mkdocs --version
python -m mkdocs --help
```

### Limpeza

```bash
# Remover site/ gerado
rm -rf site/
```

## 🎯 Boas Práticas

### 1. Estrutura Clara
- Use hierarquia lógica de pastas
- Nomes de arquivos descritivos
- URLs amigáveis (sem espaços)

### 2. Conteúdo
- Parágrafos curtos e objetivos
- Use listas para facilitar leitura
- Adicione exemplos práticos
- Inclua screenshots quando relevante

### 3. Links
- Use links relativos entre páginas
- Verifique links externos periodicamente
- Adicione `target="_blank"` para links externos

### 4. Imagens
- Otimize tamanho das imagens
- Use formatos modernos (WebP, SVG)
- Adicione alt text descritivo

### 5. Code Blocks
- Especifique a linguagem
- Use syntax highlighting
- Adicione comentários explicativos

## 📚 Recursos

- [MkDocs Documentation](https://www.mkdocs.org/)
- [Material for MkDocs](https://squidfunk.github.io/mkdocs-material/)
- [Markdown Guide](https://www.markdownguide.org/)
- [Mermaid Diagrams](https://mermaid.js.org/)

## 🐛 Troubleshooting

### Erro: "Config file not found"

```bash
# Verificar se mkdocs.yml existe
ls -la mkdocs.yml

# Executar do diretório raiz
cd /path/to/edge-video
mkdocs serve
```

### Erro: "Template not found"

```bash
# Reinstalar mkdocs-material
pip install --force-reinstall mkdocs-material
```

### Páginas não aparecem

```bash
# Verificar nav em mkdocs.yml
cat mkdocs.yml | grep -A 20 "^nav:"

# Verificar se arquivo existe
ls -la docs/path/to/file.md
```

---

**Data de criação:** 2025-11-06  
**Última atualização:** Veja rodapé das páginas
