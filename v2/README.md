# Edge Video V2 - Versão Simplificada e Otimizada

## 🎯 Arquitetura

Esta versão foi **completamente reescrita do zero** com foco em:

- ✅ **Simplicidade**: Sem abstrações desnecessárias
- ✅ **Sincronização perfeita**: Ticker preciso por câmera
- ✅ **Ordem garantida**: Captura sequencial FFmpeg
- ✅ **Zero buffers**: Sem acúmulo de frames
- ✅ **Performance**: Código otimizado e enxuto

## 📁 Estrutura

```
v2/
├── main.go         # Entrada principal
├── camera.go       # Captura RTSP com FFmpeg
├── publisher.go    # Publicação AMQP
├── config.go       # Carregamento de configuração
├── config.yaml     # Arquivo de configuração
└── edge-video-v2.exe  # Executável compilado
```

## 🔧 Como Funciona

### Captura de Frames

Cada câmera usa **FFmpeg em modo frame único**:
- Captura **exatamente 1 frame** por chamada
- Controlado por **ticker preciso** (66.67ms para 15 FPS)
- Sem buffer intermediário
- JPEG direto em memória

```go
ffmpeg -rtsp_transport tcp -i <RTSP_URL> -frames:v 1 -vcodec mjpeg -q:v 5 -f image2pipe -
```

### Publicação AMQP

- Conexão direta ao RabbitMQ
- Timestamp original preservado
- Exchange: `supercarlao_rj_mercado.exchange`
- Routing Key: `supercarlao_rj_mercado.cam1` (por câmera)

### Fluxo de Dados

```
Ticker (15 FPS)
    ↓
FFmpeg (frame único)
    ↓
JPEG em memória
    ↓
RabbitMQ Publish
    ↓
Consumers
```

## 🚀 Como Usar

### 1. Executar

```bash
cd D:\Users\rafa2\Downloads\edge-video-1.2\v2
.\edge-video-v2.exe
```

### 2. Com config customizado

```bash
.\edge-video-v2.exe -config meu-config.yaml
```

### 3. Visualizar frames

Use o viewer sincronizado:

```bash
python ..\viewer_cam1_sync.py
```

## ⚙️ Configuração

Edite `config.yaml`:

```yaml
fps: 15              # Frames por segundo
quality: 5           # Qualidade JPEG (2=melhor, 31=pior)

amqp:
  url: "amqp://..."
  exchange: "supercarlao_rj_mercado.exchange"
  routing_key_prefix: "supercarlao_rj_mercado."

cameras:
  - id: "cam1"
    url: "rtsp://..."
```

## 📊 Estatísticas

O sistema exibe estatísticas a cada 30 segundos:

```
============================================================
ESTATÍSTICAS
============================================================
Publisher: 450 publicados, 0 erros (0.00%)
[cam1] OK - Frames: 90, Último: 0s atrás
[cam2] OK - Frames: 90, Último: 0s atrás
[cam3] OK - Frames: 90, Último: 0s atrás
[cam4] OK - Frames: 90, Último: 0s atrás
[cam5] OK - Frames: 90, Último: 0s atrás
============================================================
```

## 🎯 Vantagens sobre V1

| Aspecto | V1 | V2 |
|---------|----|----|
| **Linhas de código** | ~3000 | ~400 |
| **Arquivos Go** | 15+ | 4 |
| **Worker pools** | Compartilhado | Não usa |
| **Buffers** | Múltiplos | Zero |
| **Sincronização** | Instável | Perfeita |
| **Captura** | Stream contínuo | Frame único |
| **Complexidade** | Alta | Baixa |

## 🔍 Troubleshooting

### Frames dessincronizados
- V2 não tem esse problema! Captura frame-a-frame com ticker preciso

### FFmpeg não encontrado
```bash
# Verifique se FFmpeg está no PATH
ffmpeg -version
```

### Erro de conexão AMQP
- Verifique credenciais em `config.yaml`
- Teste conectividade: `telnet 34.71.212.239 5672`

## 📝 Logs

Logs importantes:
- `[camX] Frame #N` - Frame capturado
- `Conectado ao RabbitMQ` - Conexão estabelecida
- `Sistema iniciado com sucesso!` - Tudo OK

## 🛠️ Desenvolvimento

### Recompilar

```bash
go build -o edge-video-v2.exe .
```

### Adicionar câmera

Edite `config.yaml`:

```yaml
cameras:
  - id: "cam6"
    url: "rtsp://nova-camera"
```

## 💡 Filosofia do Design

Esta versão segue os princípios:

1. **KISS** (Keep It Simple, Stupid)
2. **YAGNI** (You Aren't Gonna Need It)
3. **DRY** (Don't Repeat Yourself)

Resultado: código enxuto, rápido e confiável! 🚀
