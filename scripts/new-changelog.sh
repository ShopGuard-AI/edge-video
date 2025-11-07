#!/usr/bin/env bash
# Script helper para criar changelog fragments
# Uso: ./scripts/new-changelog.sh <tipo> "mensagem"

set -euo pipefail

# Cores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Função de ajuda
show_help() {
    cat << EOF
📝 Helper para Criar Changelog Fragments

Uso: $0 <tipo> "mensagem" [issue-number]

Tipos disponíveis:
  feature       ✨ Nova funcionalidade
  bugfix        🐛 Correção de bug
  docs          📚 Mudanças na documentação
  removal       🗑️  Remoções e depreciações
  security      🔒 Correções de segurança
  performance   ⚡ Melhorias de performance
  refactor      ♻️  Refatoração de código
  misc          🔧 Outras mudanças

Exemplos:
  $0 feature "Adiciona suporte a PostgreSQL"
  $0 bugfix "Corrige memory leak" 123
  $0 docs "Atualiza README com novos exemplos"

Opções:
  -h, --help    Mostra esta mensagem de ajuda
  -l, --list    Lista fragments existentes

EOF
}

# Listar fragments
list_fragments() {
    echo -e "${BLUE}📋 Fragments existentes:${NC}\n"
    if [ -d "changelog.d" ] && [ "$(ls -A changelog.d/*.md 2>/dev/null)" ]; then
        for file in changelog.d/*.md; do
            [ -f "$file" ] || continue
            filename=$(basename "$file")
            type=$(echo "$filename" | cut -d'.' -f2)
            content=$(head -n1 "$file")
            
            case $type in
                feature)     emoji="✨" ;;
                bugfix)      emoji="🐛" ;;
                docs)        emoji="📚" ;;
                removal)     emoji="🗑️" ;;
                security)    emoji="🔒" ;;
                performance) emoji="⚡" ;;
                refactor)    emoji="♻️" ;;
                misc)        emoji="🔧" ;;
                *)           emoji="📄" ;;
            esac
            
            echo -e "${emoji} ${GREEN}${filename}${NC}: ${content}"
        done
    else
        echo -e "${YELLOW}Nenhum fragment encontrado em changelog.d/${NC}"
    fi
    echo ""
}

# Validar tipo
validate_type() {
    local type=$1
    case $type in
        feature|bugfix|docs|removal|security|performance|refactor|misc)
            return 0
            ;;
        *)
            echo -e "${RED}❌ Erro: Tipo inválido '$type'${NC}" >&2
            echo -e "${YELLOW}Tipos válidos: feature, bugfix, docs, removal, security, performance, refactor, misc${NC}" >&2
            return 1
            ;;
    esac
}

# Processar argumentos
if [ $# -eq 0 ]; then
    show_help
    exit 1
fi

# Opções
case "${1:-}" in
    -h|--help)
        show_help
        exit 0
        ;;
    -l|--list)
        list_fragments
        exit 0
        ;;
esac

# Validar argumentos
if [ $# -lt 2 ]; then
    echo -e "${RED}❌ Erro: Argumentos insuficientes${NC}" >&2
    echo -e "${YELLOW}Uso: $0 <tipo> \"mensagem\" [issue-number]${NC}" >&2
    exit 1
fi

TYPE=$1
MESSAGE=$2
ISSUE_NUMBER=${3:-$(date +%s)}

# Validar tipo
if ! validate_type "$TYPE"; then
    exit 1
fi

# Criar diretório se não existir
mkdir -p changelog.d

# Nome do arquivo
FILENAME="changelog.d/${ISSUE_NUMBER}.${TYPE}.md"

# Verificar se já existe
if [ -f "$FILENAME" ]; then
    echo -e "${YELLOW}⚠️  Aviso: Arquivo ${FILENAME} já existe${NC}"
    read -p "Deseja sobrescrever? (s/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Ss]$ ]]; then
        echo -e "${BLUE}ℹ️  Operação cancelada${NC}"
        exit 0
    fi
fi

# Criar fragment
echo "$MESSAGE" > "$FILENAME"

# Emoji para o tipo
case $TYPE in
    feature)     emoji="✨" ;;
    bugfix)      emoji="🐛" ;;
    docs)        emoji="📚" ;;
    removal)     emoji="🗑️" ;;
    security)    emoji="🔒" ;;
    performance) emoji="⚡" ;;
    refactor)    emoji="♻️" ;;
    misc)        emoji="🔧" ;;
esac

# Sucesso
echo -e "${GREEN}✅ Fragment criado com sucesso!${NC}"
echo -e "${emoji} ${BLUE}Arquivo:${NC} $FILENAME"
echo -e "${BLUE}Conteúdo:${NC} $MESSAGE"
echo ""
echo -e "${YELLOW}💡 Próximos passos:${NC}"
echo -e "   1. ${BLUE}git add $FILENAME${NC}"
echo -e "   2. ${BLUE}git commit -m \"${TYPE}: ${MESSAGE}\"${NC}"
echo ""
