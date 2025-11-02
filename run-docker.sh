#!/bin/bash
# Script para executar o Edge Video usando Docker Run
# Uso: ./run-docker.sh /path/para/config.yaml

set -e

# Cores para output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Parâmetros
CONFIG_PATH=${1:-"$(pwd)/config.yaml"}
RABBITMQ_USER=${RABBITMQ_USER:-"user"}
RABBITMQ_PASS=${RABBITMQ_PASS:-"password"}
RABBITMQ_VHOST=${RABBITMQ_VHOST:-"guard_vhost"}
NETWORK_NAME="edge-video-net"
RABBITMQ_CONTAINER="rabbitmq"
COLLECTOR_CONTAINER="camera-collector"

echo -e "${GREEN}=== Edge Video - Docker Run Setup ===${NC}"

# Verifica se o config existe
if [ ! -f "$CONFIG_PATH" ]; then
    echo -e "${YELLOW}Erro: Config file não encontrado em: $CONFIG_PATH${NC}"
    echo "Uso: $0 /path/para/config.yaml"
    exit 1
fi

echo -e "${GREEN}[1/5] Usando config: $CONFIG_PATH${NC}"

# Cria a rede se não existir
if ! docker network inspect $NETWORK_NAME >/dev/null 2>&1; then
    echo -e "${GREEN}[2/5] Criando rede Docker: $NETWORK_NAME${NC}"
    docker network create $NETWORK_NAME
else
    echo -e "${GREEN}[2/5] Rede $NETWORK_NAME já existe${NC}"
fi

# Remove containers antigos se existirem
echo -e "${GREEN}[3/5] Limpando containers antigos...${NC}"
docker rm -f $RABBITMQ_CONTAINER 2>/dev/null || true
docker rm -f $COLLECTOR_CONTAINER 2>/dev/null || true

# Inicia RabbitMQ
echo -e "${GREEN}[4/5] Iniciando RabbitMQ...${NC}"
docker run -d \
  --name $RABBITMQ_CONTAINER \
  --network $NETWORK_NAME \
  -p 5672:5672 \
  -p 15672:15672 \
  -e RABBITMQ_DEFAULT_USER=$RABBITMQ_USER \
  -e RABBITMQ_DEFAULT_PASS=$RABBITMQ_PASS \
  -e RABBITMQ_DEFAULT_VHOST=$RABBITMQ_VHOST \
  rabbitmq:3.13-management-alpine

# Aguarda RabbitMQ iniciar
echo -e "${YELLOW}Aguardando RabbitMQ iniciar...${NC}"
sleep 10

# Inicia Camera Collector
echo -e "${GREEN}[5/5] Iniciando Camera Collector...${NC}"
docker run -d \
  --name $COLLECTOR_CONTAINER \
  --network $NETWORK_NAME \
  --restart unless-stopped \
  -v "$CONFIG_PATH:/app/config.yaml:ro" \
  t3labs/edge-video:latest

echo ""
echo -e "${GREEN}=== Setup Completo! ===${NC}"
echo ""
echo "Serviços rodando:"
echo "  - RabbitMQ Management: http://localhost:15672 (user: $RABBITMQ_USER, pass: $RABBITMQ_PASS)"
echo "  - AMQP Port: localhost:5672"
echo ""
echo "Comandos úteis:"
echo "  docker logs -f $COLLECTOR_CONTAINER  # Ver logs do collector"
echo "  docker logs -f $RABBITMQ_CONTAINER   # Ver logs do RabbitMQ"
echo "  docker stop $COLLECTOR_CONTAINER $RABBITMQ_CONTAINER  # Parar serviços"
echo "  docker rm $COLLECTOR_CONTAINER $RABBITMQ_CONTAINER    # Remover containers"
echo ""
