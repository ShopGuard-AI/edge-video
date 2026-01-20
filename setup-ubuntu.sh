#!/bin/bash
# Edge Video V2 - Setup Script para Ubuntu/Debian
# Instala FFmpeg e configura o ambiente automaticamente

set -e  # Exit on error

echo "========================================"
echo "  Edge Video V2 - Setup Ubuntu"
echo "========================================"
echo ""

# Cores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Verificar se está rodando como root
if [ "$EUID" -eq 0 ]; then
    echo -e "${YELLOW}⚠ Este script não deve ser executado como root${NC}"
    echo "Execute sem sudo. O script pedirá senha quando necessário."
    exit 1
fi

# 1. Verificar se FFmpeg já está instalado
echo -e "${YELLOW}[1/4] Verificando FFmpeg...${NC}"
if command -v ffmpeg &> /dev/null; then
    echo -e "${GREEN}✓ FFmpeg já está instalado!${NC}"
    ffmpeg -version | head -n 1
    echo ""
    read -p "Deseja reinstalar FFmpeg? (s/N): " response
    if [[ ! "$response" =~ ^[Ss]$ ]]; then
        echo -e "${GREEN}Pulando instalação do FFmpeg.${NC}"
        SKIP_FFMPEG=1
    fi
fi

# 2. Instalar FFmpeg
if [ -z "$SKIP_FFMPEG" ]; then
    echo -e "${YELLOW}[2/4] Instalando FFmpeg...${NC}"

    # Detectar distro
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        DISTRO=$ID
    else
        DISTRO="unknown"
    fi

    case "$DISTRO" in
        ubuntu|debian)
            echo "  Detectado: Ubuntu/Debian"
            sudo apt update
            sudo apt install -y ffmpeg
            ;;
        centos|rhel|fedora)
            echo "  Detectado: CentOS/RHEL/Fedora"
            sudo yum install -y ffmpeg
            ;;
        arch|manjaro)
            echo "  Detectado: Arch/Manjaro"
            sudo pacman -S --noconfirm ffmpeg
            ;;
        *)
            echo -e "${RED}✗ Distribuição não suportada: $DISTRO${NC}"
            echo "Instale FFmpeg manualmente: https://ffmpeg.org/download.html"
            exit 1
            ;;
    esac

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}  ✓ FFmpeg instalado com sucesso!${NC}"
    else
        echo -e "${RED}  ✗ Erro ao instalar FFmpeg${NC}"
        exit 1
    fi
else
    echo -e "${YELLOW}[2/4] FFmpeg - pulado${NC}"
fi

# 3. Tornar producer executável
echo -e "${YELLOW}[3/4] Configurando permissões...${NC}"
if [ -f "producer-linux" ]; then
    chmod +x producer-linux
    echo -e "${GREEN}  ✓ producer-linux é executável${NC}"
elif [ -f "producer" ]; then
    chmod +x producer
    echo -e "${GREEN}  ✓ producer é executável${NC}"
else
    echo -e "${RED}  ✗ Binário 'producer-linux' ou 'producer' não encontrado!${NC}"
    echo "  Certifique-se de ter extraído todos os arquivos do release."
    exit 1
fi

# 4. Validar instalação
echo -e "${YELLOW}[4/4] Validando instalação...${NC}"

# Verificar FFmpeg
if command -v ffmpeg &> /dev/null; then
    FFMPEG_VERSION=$(ffmpeg -version | head -n 1)
    echo -e "${GREEN}  ✓ FFmpeg: $FFMPEG_VERSION${NC}"
else
    echo -e "${RED}  ✗ FFmpeg não encontrado no PATH${NC}"
    exit 1
fi

# Verificar producer
if [ -f "producer-linux" ]; then
    PRODUCER_SIZE=$(du -h producer-linux | cut -f1)
    echo -e "${GREEN}  ✓ Producer: producer-linux ($PRODUCER_SIZE)${NC}"
    PRODUCER_BIN="./producer-linux"
elif [ -f "producer" ]; then
    PRODUCER_SIZE=$(du -h producer | cut -f1)
    echo -e "${GREEN}  ✓ Producer: producer ($PRODUCER_SIZE)${NC}"
    PRODUCER_BIN="./producer"
fi

echo ""
echo "========================================"
echo "  Setup Concluído!"
echo "========================================"
echo ""
echo -e "${GREEN}Próximos Passos:${NC}"
echo ""
echo "1. Edite config.yaml com suas configurações:"
echo "   nano config.yaml"
echo ""
echo "2. Execute o producer:"
echo "   $PRODUCER_BIN"
echo ""
echo "3. Para instalar como serviço systemd:"
echo "   Consulte DEPLOY_UBUNTU.md"
echo ""
echo -e "${GREEN}✓ Tudo pronto para começar!${NC}"
echo ""
