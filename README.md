# Edge Video - Sistema de Captura e Distribuição de Vídeo

## 📋 Objetivo do Projeto

O **Edge Video** é um sistema distribuído de captura e streaming de câmeras RTSP, projetado para ambientes de edge computing. O sistema captura frames de múltiplas câmeras IP em tempo real, processa-os e distribui através de uma fila de mensagens (RabbitMQ), permitindo que múltiplos consumidores recebam e processem os streams de vídeo de forma escalável e eficiente.

## 🎯 Principais Funcionalidades

- **Captura Multi-Câmera**: Suporta a captura simultânea de múltiplas câmeras RTSP/IP
- **Processamento em Edge**: Processamento local dos frames antes da transmissão
- **Distribuição via Message Broker**: Utiliza RabbitMQ com protocolo AMQP para distribuição eficiente
- **Visualização em Grid**: Interface Python para visualização de todas as câmeras em uma única janela
- **Configuração Flexível**: Fácil adição/remoção de câmeras via arquivo YAML
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

## 🛠️ Tecnologias Utilizadas

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
├── config.yaml              # Configuração das câmeras e parâmetros
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

Edite o arquivo `config.yaml` e adicione as URLs das suas câmeras:

```yaml
cameras:
  - id: "cam1"
    url: "rtsp://user:pass@192.168.1.100:554/stream"
  - id: "cam2"
    url: "rtsp://user:pass@192.168.1.101:554/stream"
  # ... até 6 câmeras
```

### 2. Inicie os Serviços

```bash
docker-compose up -d --build
```

Isso iniciará:
- **RabbitMQ**: Porta 5672 (AMQP) e 15672 (Management UI)
- **Camera Collector**: Aplicação Go capturando e publicando frames

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
2. Crie uma branch para sua feature
3. Commit suas mudanças
4. Push para a branch
5. Abra um Pull Request

## 📝 Licença

Este projeto está sob a licença MIT.

## 🔗 Links

- **Repositório**: https://github.com/T3-Labs/edge-video
- **RabbitMQ**: https://www.rabbitmq.com/
- **FFmpeg**: https://ffmpeg.org/
- **OpenCV**: https://opencv.org/

---

**Desenvolvido por T3 Labs** 🚀
