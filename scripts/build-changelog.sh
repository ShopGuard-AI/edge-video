#!/usr/bin/env bash
# Script helper para gerar CHANGELOG com Towncrier
# Uso: ./scripts/build-changelog.sh <versão>

set -euo pipefail

# Cores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

show_help() {
    cat << EOF
📚 Helper para Gerar CHANGELOG

Uso: $0 [opções] <versão>

Opções:
  -h, --help      Mostra esta mensagem
  -d, --draft     Preview sem modificar arquivos
  -k, --keep      Mantém os fragments após gerar
  -y, --yes       Não pede confirmação

Exemplos:
  $0 1.0.0                    # Gera changelog para v1.0.0
  $0 --draft 1.0.0            # Preview do changelog
  $0 --keep 1.0.0             # Gera mas mantém fragments
  $0 --yes 1.0.0              # Gera sem pedir confirmação

EOF
}

# Defaults
DRAFT=false
KEEP=false
YES=false
VERSION=""

# Processar argumentos
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -d|--draft)
            DRAFT=true
            shift
            ;;
        -k|--keep)
            KEEP=true
            shift
            ;;
        -y|--yes)
            YES=true
            shift
            ;;
        *)
            VERSION=$1
            shift
            ;;
    esac
done

# Validar versão
if [ -z "$VERSION" ]; then
    echo -e "${RED}❌ Erro: Versão não especificada${NC}" >&2
    echo -e "${YELLOW}Uso: $0 <versão>${NC}" >&2
    exit 1
fi

# Remover 'v' prefixo se existir
VERSION=${VERSION#v}

# Verificar se towncrier está instalado
if ! command -v towncrier &> /dev/null; then
    echo -e "${RED}❌ Erro: towncrier não está instalado${NC}" >&2
    echo -e "${YELLOW}Instale com: pip install towncrier${NC}" >&2
    exit 1
fi

# Contar fragments
FRAGMENT_COUNT=$(find changelog.d -name "*.md" ! -name "README.md" ! -name "template.md.j2" 2>/dev/null | wc -l)

if [ "$FRAGMENT_COUNT" -eq 0 ]; then
    echo -e "${YELLOW}⚠️  Aviso: Nenhum fragment encontrado em changelog.d/${NC}"
    echo -e "${BLUE}ℹ️  Nada para adicionar ao CHANGELOG${NC}"
    exit 0
fi

echo -e "${BLUE}📋 Fragments encontrados: ${FRAGMENT_COUNT}${NC}"
echo ""

# Listar fragments
echo -e "${BLUE}Fragments que serão processados:${NC}"
for file in changelog.d/*.md; do
    [ -f "$file" ] || continue
    [[ "$file" == *"README.md" ]] && continue
    [[ "$file" == *"template.md.j2" ]] && continue
    
    filename=$(basename "$file")
    content=$(head -n1 "$file")
    type=$(echo "$filename" | cut -d'.' -f2)
    
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
    
    echo -e "  ${emoji} ${GREEN}${filename}${NC}: ${content}"
done
echo ""

# Construir comando
CMD="towncrier build --version ${VERSION}"

if [ "$DRAFT" = true ]; then
    CMD="$CMD --draft"
    echo -e "${YELLOW}🔍 Modo DRAFT: Nenhum arquivo será modificado${NC}"
    echo ""
fi

if [ "$KEEP" = true ]; then
    CMD="$CMD --keep"
fi

# Confirmação
if [ "$YES" = false ] && [ "$DRAFT" = false ]; then
    echo -e "${YELLOW}⚠️  Isso irá:${NC}"
    echo -e "   1. Atualizar ${BLUE}CHANGELOG.md${NC} com as mudanças acima"
    if [ "$KEEP" = false ]; then
        echo -e "   2. ${RED}Remover${NC} os fragments processados"
    else
        echo -e "   2. ${GREEN}Manter${NC} os fragments (--keep ativado)"
    fi
    echo ""
    read -p "Continuar? (s/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Ss]$ ]]; then
        echo -e "${BLUE}ℹ️  Operação cancelada${NC}"
        exit 0
    fi
fi

# Executar towncrier
echo -e "${BLUE}🔨 Gerando CHANGELOG...${NC}"
echo ""

if $CMD; then
    if [ "$DRAFT" = true ]; then
        echo ""
        echo -e "${GREEN}✅ Preview gerado com sucesso!${NC}"
        echo -e "${YELLOW}💡 Para gerar de verdade, execute sem --draft${NC}"
    else
        echo ""
        echo -e "${GREEN}✅ CHANGELOG gerado com sucesso!${NC}"
        echo ""
        echo -e "${YELLOW}💡 Próximos passos:${NC}"
        echo -e "   1. ${BLUE}git add CHANGELOG.md${NC}"
        if [ "$KEEP" = false ]; then
            echo -e "   2. ${BLUE}git add changelog.d/${NC} (fragments removidos)"
        fi
        echo -e "   3. ${BLUE}git commit -m \"chore: release v${VERSION}\"${NC}"
        echo -e "   4. ${BLUE}git tag -a v${VERSION} -m \"Release v${VERSION}\"${NC}"
        echo -e "   5. ${BLUE}git push origin main --tags${NC}"
    fi
else
    echo -e "${RED}❌ Erro ao gerar CHANGELOG${NC}" >&2
    exit 1
fi
