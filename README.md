# Edge Video - Sistema de Captura e Distribuição de Vídeo

[![Go Tests](https://github.com/T3-Labs/edge-video/actions/workflows/go-test.yml/badge.svg)](https://github.com/T3-Labs/edge-video/actions/workflows/go-test.yml)
[![Docker Build](https://github.com/T3-Labs/edge-video/actions/workflows/build-and-push.yml/badge.svg)](https://github.com/T3-Labs/edge-video/actions/workflows/build-and-push.yml)
[![Go Version](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## 📋 Objetivo do Projeto

O **Edge Video** é um sistema distribuído de captura e streaming de câmeras RTSP, projetado para ambientes de edge computing. O sistema captura frames de múltiplas câmeras IP em tempo real, processa-os e distribui através de uma fila de mensagens (RabbitMQ), permitindo que múltiplos consumidores recebam e processem os streams de vídeo de forma escalável e eficiente.

## 🎯 Principais Funcionalidades

- **Captura Multi-Câmera**: Suporta a captura simultânea de múltiplas câmeras RTSP/IP
- **Processamento em Edge**: Processamento local dos frames antes da transmissão
- **Distribuição via Message Broker**: Utiliza RabbitMQ com protocolo AMQP para distribuição eficiente
- **Visualização em Grid**: Interface Python para visualização de todas as câmeras em uma única janela
- **Configuração Flexível**: Fácil adição/remoção de câmeras via arquivo TOML
- **Containerizado**: Deploy simplificado com Docker e Docker Compose

## 🏗️ Arquitetura

```
┌─────────────────┐
│  Câmeras RTSP   │
│  (5 câmeras)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Camera         │
│  Collector      │  ← Aplicação Go
│  (FFmpeg)       │
└────────┬────────┘
         │ JPEG Frames
         ▼
┌─────────────────┐
│   RabbitMQ      │
│   (AMQP)        │
│   Exchange:     │
│   cameras       │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│                 │
│    Consumer     │  ← Visualização em Grid 2x3
│                 │
└─────────────────┘
```

## � Código Refatorado

Este repositório foi refatorado seguindo as melhores práticas de desenvolvimento Python:

### **Estrutura Refatorada:**
```
src/
├── config/
│   └── config_manager.py      # Gerenciamento de configuração
├── consumer/
│   └── rabbitmq_consumer.py   # Consumidor RabbitMQ
├── display/
│   ├── display_manager.py     # Gerenciador de display OpenCV
│   └── video_processor.py     # Processamento de frames
└── video_consumer_app.py      # Aplicação principal

tests/
├── test_config_manager.py
├── test_rabbitmq_consumer.py
├── test_display_manager.py
├── test_video_processor.py
└── test_video_consumer_app.py
```

### **Principais Melhorias:**
- **Single Responsibility Principle**: Cada classe tem uma responsabilidade específica
- **Separação de Concerns**: Lógica de negócio separada da apresentação
- **Testabilidade**: 100% de cobertura de testes unitários
- **Type Hints**: Tipagem completa para melhor manutenibilidade
- **Documentação**: Docstrings detalhadas seguindo padrões Python

### **Como usar o código refatorado:**
```bash
# Instalar dependências
uv sync --dev

# Executar testes
uv run pytest

# Executar aplicação refatorada
uv run python main_refactored.py

# Executar linting
uv run ruff check src/
uv run ruff format src/
```

## �🛠️ Tecnologias Utilizadas

### Backend (Collector)
- **Go 1.24**: Linguagem principal para o collector
- **FFmpeg**: Captura de frames das câmeras RTSP
- **Viper**: Gerenciamento de configuração
- **AMQP (streadway/amqp)**: Cliente RabbitMQ

### Message Broker
- **RabbitMQ 3.13**: Sistema de mensageria para distribuição de frames

### Frontend (Consumer)
- **Python 3.11+**: Linguagem para o consumer
- **OpenCV**: Processamento e visualização de vídeo
- **Pika**: Cliente RabbitMQ para Python
- **NumPy**: Manipulação de arrays para concatenação de frames

### Infraestrutura
- **Docker & Docker Compose**: Containerização e orquestração
- **Alpine Linux**: Imagem base leve para containers

## 📦 Estrutura do Projeto

```
edge_guard_ai/
├── config.toml              # Configuração das câmeras e parâmetros
├── docker-compose.yml       # Orquestração dos serviços
├── Dockerfile              # Build da aplicação Go
├── main.go                 # Entrypoint da aplicação
├── go.mod                  # Dependências Go
├── pyproject.toml          # Dependências Python
├── test_consumer.py        # Consumer Python com visualização
├── internal/
│   ├── camera/
│   │   └── camera.go       # Lógica de captura de frames
│   ├── mq/
│   │   ├── publisher.go    # Interface do publisher
│   │   ├── amqp.go         # Implementação AMQP
│   │   └── mqtt.go         # Implementação MQTT (alternativa)
│   └── util/
│       └── compress.go     # Utilitários de compressão
└── README.md               # Este arquivo
```

## 🚀 Como Executar

### Pré-requisitos

- Docker e Docker Compose instalados
- Python 3.11+ (para o consumer)
- UV (gerenciador de pacotes Python) ou pip

### 1. Configure as Câmeras

Edite o arquivo `config.toml` e adicione as URLs das suas câmeras:

```toml
[[cameras]]
id = "cam1"
url = "rtsp://user:pass@192.168.1.100:554/stream"

[[cameras]]
id = "cam2"
url = "rtsp://user:pass@192.168.1.101:554/stream"

# ... até 6 câmeras
```

### 2. Executar a Aplicação

#### Usando arquivo de configuração padrão

```bash
# Compilar e executar
go build -o edge-video ./cmd/edge-video
./edge-video

# Ou executar diretamente
go run ./cmd/edge-video
```

#### Usando arquivo de configuração customizado

```bash
# Especificar arquivo via parâmetro --config
./edge-video --config /path/to/custom-config.toml

# Ou com go run
go run ./cmd/edge-video --config config.test.toml
```

#### Validar configuração

```bash
# Validar arquivo de configuração
go run ./cmd/validate-config --config config.toml

# Ver ajuda
./edge-video --help
# Output:
#   -config string
#         Caminho para o arquivo de configuração (default "config.toml")
```

### 3. Inicie os Serviços com Docker

#### Opção A: Usando Docker Compose (Recomendado)

```bash
docker-compose up -d --build
```

Isso iniciará:
- **RabbitMQ**: Porta 5672 (AMQP) e 15672 (Management UI)
- **Camera Collector**: Aplicação Go capturando e publicando frames

#### Opção B: Usando Docker Run (Após Docker Pull)

Se você baixou a imagem do Docker Hub com `docker pull`:

```bash
# 1. Inicie o RabbitMQ primeiro
docker run -d \
  --name rabbitmq \
  -p 5672:5672 \
  -p 15672:15672 \
  -e RABBITMQ_DEFAULT_USER=user \
  -e RABBITMQ_DEFAULT_PASS=password \
  -e RABBITMQ_DEFAULT_VHOST=guard_vhost \
  rabbitmq:3.13-management-alpine

# 2. Baixe a imagem do Edge Video (se ainda não tiver)
docker pull t3labs/edge-video:latest

# 3. Execute o Camera Collector com seu config.toml local
docker run -d \
  --name camera-collector \
  --link rabbitmq:rabbitmq \
  -v /path/absoluto/para/seu/config.toml:/app/config.toml \
  t3labs/edge-video:latest
```

**Exemplos de caminhos para o volume:**

```bash
# Exemplo 1: Config.toml na pasta atual
docker run -d \
  --name camera-collector \
  --link rabbitmq:rabbitmq \
  -v $(pwd)/config.toml:/app/config.toml \
  t3labs/edge-video:latest

# Exemplo 2: Config.toml em /etc
docker run -d \
  --name camera-collector \
  --link rabbitmq:rabbitmq \
  -v /etc/edge-video/config.toml:/app/config.toml \
  t3labs/edge-video:latest

# Exemplo 3: Config.toml no home do usuário
docker run -d \
  --name camera-collector \
  --link rabbitmq:rabbitmq \
  -v $HOME/.config/edge-video/config.toml:/app/config.toml \
  t3labs/edge-video:latest

# Exemplo 4: Config.toml em storage montado
docker run -d \
  --name camera-collector \
  --link rabbitmq:rabbitmq \
  -v /mnt/storage/configs/cameras.toml:/app/config.toml \
  t3labs/edge-video:latest
```

**Usando Docker Network (Melhor prática):**

```bash
# 1. Crie uma rede
docker network create edge-video-net

# 2. Inicie o RabbitMQ na rede
docker run -d \
  --name rabbitmq \
  --network edge-video-net \
  -p 5672:5672 \
  -p 15672:15672 \
  -e RABBITMQ_DEFAULT_USER=user \
  -e RABBITMQ_DEFAULT_PASS=password \
  -e RABBITMQ_DEFAULT_VHOST=guard_vhost \
  rabbitmq:3.13-management-alpine

# 3. Execute o Camera Collector na mesma rede
docker run -d \
  --name camera-collector \
  --network edge-video-net \
  -v /path/para/seu/config.yaml:/app/config.yaml \
  t3labs/edge-video:latest
```

### 3. Execute o Consumer Python

```bash
# Com UV
uv run test_consumer.py

# Ou com pip
pip install -r requirements.txt
python test_consumer.py
```

### 4. Visualize as Câmeras

Uma janela será aberta mostrando todas as câmeras em uma grade 2x3.

**Pressione 'q' para sair.**

## ⚙️ Configuração

### config.yaml

```yaml
interval_ms: 500                    # Intervalo entre capturas (ms)
protocol: amqp                      # Protocolo: amqp ou mqtt
process_every_n_frames: 3           # Reduz taxa de frames (1 a cada 3)

amqp:
  amqp_url: "amqp://user:password@rabbitmq:5672/guard_vhost"
  exchange: "cameras"
  routing_key_prefix: "camera"

compression:
  enabled: false                    # Compressão zstd (desabilitada)
  level: 3

cameras:
  - id: "cam1"
    url: "rtsp://..."
  - id: "cam2"
    url: "rtsp://..."
```

### 🔄 Optional Redis Frame Storage + Metadata

You can enable Redis frame caching and metadata publishing by updating `config.yaml`:

```yaml
redis:
  enabled: true
  address: "redis:6379"
  ttl_seconds: 300
  prefix: "frames"

metadata:
  enabled: true
  exchange: "camera.metadata"
  routing_key: "camera.metadata.event"
```

When enabled:

- Frames are stored in Redis with TTL
- Metadata messages are sent asynchronously to RabbitMQ
- Existing video streaming and publishing are unaffected

## 🔍 Monitoramento

### RabbitMQ Management UI

Acesse: `http://localhost:15672`
- **Usuário**: user
- **Senha**: password

### Logs do Collector

```bash
docker logs camera-collector -f
```

### Métricas do Sistema

Verifique o throughput de mensagens e o uso de recursos no RabbitMQ Management.

## 📊 Casos de Uso

1. **Vigilância e Segurança**: Monitoramento em tempo real de múltiplas câmeras
2. **Análise de Vídeo**: Processamento de frames para detecção de objetos, pessoas, etc.
3. **Edge Computing**: Processamento local antes de envio para a nuvem
4. **Sistemas de Visão Computacional**: Pipeline para aplicações de Computer Vision
5. **Armazenamento Inteligente**: Gravação seletiva baseada em eventos

## 🔧 Desenvolvimento

### Adicionar Nova Câmera

1. Edite `config.yaml`
2. Adicione a nova entrada em `cameras`
3. Reinicie o container: `docker-compose restart camera-collector`

### Modificar Taxa de Frames

Ajuste `interval_ms` no `config.yaml` para controlar a taxa de captura.

### Habilitar Compressão

```yaml
compression:
  enabled: true
  level: 3  # 1-22 (maior = mais compressão)
```

## 🤝 Contribuindo

Este é um projeto da **T3 Labs**. Para contribuir:

1. Fork o repositório
2. Crie uma branch para sua feature (`git checkout -b feature/nova-funcionalidade`)
3. **Crie um changelog fragment** para suas mudanças:
   ```bash
   ./scripts/new-changelog.sh feature "Descrição da mudança"
   ```
4. Commit suas mudanças usando [commits semânticos](https://www.conventionalcommits.org/):
   ```bash
   git commit -m "feat: adiciona nova funcionalidade"
   ```
5. Push para a branch (`git push origin feature/nova-funcionalidade`)
6. Abra um Pull Request

### 📝 Sistema de Changelog

Este projeto usa [Towncrier](https://towncrier.readthedocs.io/) para gerenciar o changelog automaticamente.

**Criar um fragment:**
```bash
# Usando o script helper (recomendado)
./scripts/new-changelog.sh feature "Adiciona suporte a PostgreSQL"

# Ou manualmente
echo "Adiciona suporte a PostgreSQL" > changelog.d/$(date +%s).feature.md
```

**Tipos disponíveis:** `feature`, `bugfix`, `docs`, `removal`, `security`, `performance`, `refactor`, `misc`

**Gerar changelog para release:**
```bash
# Preview
./scripts/build-changelog.sh --draft 1.0.0

# Gerar
./scripts/build-changelog.sh 1.0.0
```

Para mais detalhes, veja [docs/PRECOMMIT_TOWNCRIER_GUIDE.md](docs/PRECOMMIT_TOWNCRIER_GUIDE.md)

### 🔍 Pre-commit Hooks

Este projeto usa pre-commit hooks para garantir qualidade:

```bash
# Instalar hooks
pip install pre-commit towncrier
pre-commit install
pre-commit install --hook-type commit-msg

# Executar manualmente
pre-commit run --all-files
```

Os hooks verificam:
- ✅ Formatação de código Go (gofmt, goimports)
- ✅ Lint (go vet, golangci-lint)
- ✅ Changelog fragments (towncrier)
- ✅ Formato de commits (commitizen)
- ✅ Detecção de segredos
- ✅ Validação de YAML/TOML/JSON

## 📝 Licença

Este projeto está sob a licença MIT.

## 🔗 Links

- **Repositório**: https://github.com/T3-Labs/edge-video
- **RabbitMQ**: https://www.rabbitmq.com/
- **FFmpeg**: https://ffmpeg.org/
- **OpenCV**: https://opencv.org/

---

**Desenvolvido por T3 Labs** 🚀
