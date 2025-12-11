# Edge Video V2 - Deploy Windows (Serviço Resiliente)

Guia completo para instalar Edge Video V2 como serviço Windows resiliente, com auto-restart e recuperação automática de falhas.

---

## 🚀 Instalação Rápida (Recomendada)

### 1. Download do Release

Baixe o arquivo ZIP do release: `edge-video-v1.6.zip`

### 2. Extrair e Rodar Setup

```powershell
# Extrair para C:\EdgeVideo
Expand-Archive -Path edge-video-v1.6.zip -DestinationPath C:\EdgeVideo
cd C:\EdgeVideo

# Rodar setup automático (baixa FFmpeg)
.\setup.ps1
```

O script `setup.ps1` faz automaticamente:
- ✅ Baixa FFmpeg (~100 MB)
- ✅ Extrai apenas o necessário
- ✅ Valida instalação
- ✅ Limpa arquivos temporários

### 3. Configurar e Rodar

```powershell
# Edite config.yaml com suas credenciais
notepad config.yaml

# Teste manualmente
.\producer.exe
```

**Pronto!** Se funcionou, vá para seção "Instalar como Serviço Windows" abaixo.

---

## 📋 Instalação Manual (Alternativa)

Se o `setup.ps1` falhar (firewall, antivírus), instale manualmente:

### 1. FFmpeg (OBRIGATÓRIO)

**Download**: https://github.com/BtbN/FFmpeg-Builds/releases

1. Baixe: `ffmpeg-master-latest-win64-gpl.zip`
2. Extraia `bin/ffmpeg.exe` para: `C:\EdgeVideo\ffmpeg.exe`

OU adicione ao PATH do sistema:
   ```powershell
   # Como Administrador
   [Environment]::SetEnvironmentVariable("Path", $env:Path + ";C:\ffmpeg\bin", "Machine")
   ```

Verifique:
   ```powershell
   ffmpeg -version
   ```

### 2. NSSM (Non-Sucking Service Manager)

**Download**: https://nssm.cc/download

1. Baixe: `nssm-2.24.zip`
2. Extraia para: `C:\nssm`
3. Use: `C:\nssm\win64\nssm.exe`

---

## 📦 Instalação

### 1. Preparar Diretório

```powershell
# Criar diretório de instalação
mkdir C:\EdgeVideo
cd C:\EdgeVideo

# Copiar arquivos
# - producer.exe
# - config.yaml
```

### 2. Configurar `config.yaml`

```yaml
fps: 15
quality: 5

redis:
  enabled: true
  address: "35.199.96.88:6379"
  password: "SuaSenha"
  ttl: 120

amqp:
  url: "amqp://user:pass@34.71.212.239:5672/vhost"

circuit_breaker:
  enabled: true
  max_failures: 5
  reset_timeout: 30s
  initial_backoff: 5s
  max_backoff: 5m

memory_controller:
  enabled: true
  max_memory_mb: 1024

cameras:
  - id: "cam1"
    url: "rtmp://servidor:1935/rtmp/stream"
    exchange: "vhost.exchange"
    routing_key: "vhost.cam1"
```

### 3. Testar Manualmente (IMPORTANTE!)

```powershell
cd C:\EdgeVideo
.\producer.exe
```

**Verifique**:
- ✅ Conecta Redis
- ✅ Conecta RabbitMQ
- ✅ FFmpeg captura frames
- ✅ Métricas: http://localhost:2112/metrics

**Ctrl+C** para parar.

---

## 🔧 Instalar como Serviço Windows

### 1. Instalar Serviço com NSSM

```powershell
# Como Administrador
C:\nssm\win64\nssm.exe install EdgeVideoProducer C:\EdgeVideo\producer.exe
```

### 2. Configurar Parâmetros do Serviço

```powershell
# Diretório de trabalho
C:\nssm\win64\nssm.exe set EdgeVideoProducer AppDirectory C:\EdgeVideo

# Logs stdout (info)
C:\nssm\win64\nssm.exe set EdgeVideoProducer AppStdout C:\EdgeVideo\logs\stdout.log
C:\nssm\win64\nssm.exe set EdgeVideoProducer AppStderr C:\EdgeVideo\logs\stderr.log

# Rotação de logs (10 MB, 5 arquivos)
C:\nssm\win64\nssm.exe set EdgeVideoProducer AppStdoutCreationDisposition 4
C:\nssm\win64\nssm.exe set EdgeVideoProducer AppStderrCreationDisposition 4
C:\nssm\win64\nssm.exe set EdgeVideoProducer AppRotateFiles 1
C:\nssm\win64\nssm.exe set EdgeVideoProducer AppRotateOnline 1
C:\nssm\win64\nssm.exe set EdgeVideoProducer AppRotateBytes 10485760
```

### 3. Configurar Auto-Restart (RESILIENTE)

```powershell
# Restart em caso de falha (exit code != 0)
C:\nssm\win64\nssm.exe set EdgeVideoProducer AppExit Default Restart

# Delay de restart: 5 segundos
C:\nssm\win64\nssm.exe set EdgeVideoProducer AppRestartDelay 5000

# Throttle: previne restart loops (reset após 60s)
C:\nssm\win64\nssm.exe set EdgeVideoProducer AppThrottle 60000

# Start automático (boot da máquina)
C:\nssm\win64\nssm.exe set EdgeVideoProducer Start SERVICE_AUTO_START
```

### 4. Prioridade de Processo

```powershell
# Prioridade alta (melhor performance)
C:\nssm\win64\nssm.exe set EdgeVideoProducer AppPriority ABOVE_NORMAL_PRIORITY_CLASS
```

### 5. Iniciar Serviço

```powershell
# Iniciar
net start EdgeVideoProducer

# Verificar status
C:\nssm\win64\nssm.exe status EdgeVideoProducer

# Ver logs
Get-Content C:\EdgeVideo\logs\stdout.log -Tail 50
```

---

## 🛡️ Resiliência Garantida

### Recuperação Automática

✅ **Queda de rede**: Producer reconecta automaticamente (Redis + RabbitMQ)
✅ **Redis offline**: Retry automático com exponential backoff
✅ **RabbitMQ offline**: Reconexão automática com exponential backoff
✅ **Câmera offline**: Circuit Breaker isola câmera (testa a cada 30s)
✅ **FFmpeg crash**: CameraStream recria processo automaticamente
✅ **Producer crash**: NSSM reinicia serviço em 5s
✅ **Reboot máquina**: Serviço inicia automaticamente (SERVICE_AUTO_START)
✅ **Memory leak**: Memory Controller força GC (previne OOM)

### Circuit Breaker

Protege contra câmeras com falhas persistentes:
- 5 falhas → Estado OPEN (para tentativas)
- Aguarda 30s → Estado HALF_OPEN (testa reconexão)
- 3 sucessos → Estado CLOSED (volta normal)
- Backoff exponencial: 5s → 5min

### Logs Rotativos

NSSM gerencia rotação automática:
- Tamanho máximo: 10 MB por arquivo
- Mantém últimos 5 arquivos
- Rotação online (sem parar serviço)

Arquivos gerados:
```
C:\EdgeVideo\logs\
├── stdout.log          (atual)
├── stdout.log.1        (anterior)
├── stdout.log.2
├── stdout.log.3
└── stdout.log.4
```

---

## 📊 Monitoramento

### Opção 1: Verificação Rápida (CLI)

```powershell
# Status do serviço
sc query EdgeVideoProducer

# Logs em tempo real
Get-Content C:\EdgeVideo\logs\stdout.log -Wait

# Métricas Prometheus (texto bruto)
curl http://localhost:2112/metrics

# Health check
curl http://localhost:2112/health

# pprof (goroutines)
curl http://localhost:6060/debug/pprof/goroutine?debug=1
```

### Opção 2: Stack Completo Prometheus + Grafana (RECOMENDADO)

#### Instalação do Docker Desktop

1. **Download**: https://www.docker.com/products/docker-desktop
2. Instale Docker Desktop for Windows
3. Reinicie a máquina
4. Verifique instalação:
   ```powershell
   docker --version
   docker-compose --version
   ```

#### Subir Stack de Monitoramento

```powershell
# Navegar para pasta monitoring
cd C:\EdgeVideo\monitoring

# Subir Prometheus + Grafana
docker-compose up -d

# Verificar containers
docker-compose ps
```

**Serviços disponíveis**:
- 📊 **Grafana**: http://localhost:3000 (admin/admin)
- 📈 **Prometheus**: http://localhost:9090

#### Acessar Dashboard Grafana

1. Acesse: http://localhost:3000
2. Login: `admin` / `admin`
3. Dashboards → "Edge Video V2"

**Dashboard inclui**:
- 📈 FPS em tempo real por câmera
- 💾 Uso de memória e CPU
- 🔴 Circuit Breaker states
- 📊 Frames publicados vs descartados
- ⏱️ Latência de publicação
- 🎯 Taxa de ACK/NACK RabbitMQ

#### Gerenciar Stack

```powershell
# Ver logs
docker-compose logs -f

# Parar stack
docker-compose down

# Parar e remover volumes (apaga histórico)
docker-compose down -v

# Restart
docker-compose restart
```

#### Retenção de Dados

Prometheus mantém métricas por **30 dias** (configurável em `prometheus/prometheus.yml`):
```yaml
storage.tsdb.retention.time=30d
```

### Métricas Importantes

```promql
# FPS por câmera
edge_video_camera_fps{camera_id="cam1"}

# Frames publicados (taxa de sucesso)
rate(edge_video_frames_published_total[1m])

# Circuit Breaker states (0=CLOSED, 1=OPEN, 2=HALF_OPEN)
edge_video_circuit_breaker_state{camera_id="cam1"}

# Memory usage
edge_video_system_ram_mb

# Goroutines (detectar leak)
edge_video_system_goroutines

# Taxa de ACK
rate(edge_video_publisher_confirms_ack_total[5m])
/
(rate(edge_video_publisher_confirms_ack_total[5m]) + rate(edge_video_publisher_confirms_nack_total[5m]))
```

### Alertas Recomendados

Configure alertas no Prometheus Alertmanager (opcional):

1. **FPS baixo**: `edge_video_camera_fps < 8`
2. **Memory alto**: `edge_video_system_ram_mb > 900`
3. **Circuit Breaker OPEN**: `edge_video_circuit_breaker_state == 1`
4. **Goroutine leak**: `edge_video_system_goroutines > 50`
5. **Drops frequentes**: `rate(edge_video_frames_dropped_total[1m]) > 1`
6. **Taxa de ACK < 95%**: `taxa_ack < 0.95`

### Firewall (se acessar remotamente)

```powershell
# Grafana (porta 3000)
New-NetFirewallRule -DisplayName "Grafana" -Direction Inbound -LocalPort 3000 -Protocol TCP -Action Allow

# Prometheus (porta 9090)
New-NetFirewallRule -DisplayName "Prometheus" -Direction Inbound -LocalPort 9090 -Protocol TCP -Action Allow

# Métricas Producer (porta 2112)
New-NetFirewallRule -DisplayName "EdgeVideo Metrics" -Direction Inbound -LocalPort 2112 -Protocol TCP -Action Allow
```

---

## 🔧 Gerenciamento do Serviço

### Comandos Básicos

```powershell
# Iniciar
net start EdgeVideoProducer

# Parar
net stop EdgeVideoProducer

# Restart
net stop EdgeVideoProducer && net start EdgeVideoProducer

# Status
C:\nssm\win64\nssm.exe status EdgeVideoProducer

# Remover serviço
C:\nssm\win64\nssm.exe remove EdgeVideoProducer confirm
```

### Atualizar Producer

```powershell
# 1. Parar serviço
net stop EdgeVideoProducer

# 2. Backup do executável atual
copy C:\EdgeVideo\producer.exe C:\EdgeVideo\producer.exe.bak

# 3. Substituir por novo executável
copy producer.exe C:\EdgeVideo\

# 4. Iniciar serviço
net start EdgeVideoProducer

# 5. Verificar logs
Get-Content C:\EdgeVideo\logs\stdout.log -Tail 50
```

### Rollback (se atualização falhar)

```powershell
net stop EdgeVideoProducer
copy C:\EdgeVideo\producer.exe.bak C:\EdgeVideo\producer.exe
net start EdgeVideoProducer
```

---

## 🚨 Troubleshooting

### Serviço não inicia

```powershell
# Ver detalhes erro
C:\nssm\win64\nssm.exe status EdgeVideoProducer

# Testar manualmente
cd C:\EdgeVideo
.\producer.exe
```

**Causas comuns**:
- FFmpeg não no PATH → Adicione ao PATH e reinicie máquina
- config.yaml inválido → Valide YAML syntax
- Redis/RabbitMQ inacessíveis → Verifique conectividade

### FPS baixo

**Causa**: Redis remoto alta latência
**Solução**: Use Redis local ou aumente TTL

### Memory > 1 GB

**Causa**: Muitas câmeras ou quality alta
**Solução**: Reduza quality (5→3) ou max_memory_mb

### Circuit Breaker OPEN

**Causa**: Câmera offline ou URL inválida
**Solução**:
```powershell
# Teste URL com FFmpeg
ffmpeg -i "rtmp://servidor:1935/stream" -frames:v 1 test.jpg
```

### Logs não aparecem

**Causa**: Diretório logs não existe
**Solução**:
```powershell
mkdir C:\EdgeVideo\logs
net restart EdgeVideoProducer
```

---

## 📁 Estrutura de Diretórios

```
C:\EdgeVideo\
├── producer.exe           ← Executável
├── config.yaml            ← Configuração
├── setup.ps1              ← Setup automático FFmpeg
├── DEPLOY_WINDOWS.md      ← Este guia
├── producer.exe.bak       ← Backup (atualizações)
├── logs\
│   ├── stdout.log         ← Logs info
│   ├── stderr.log         ← Logs erro
│   └── *.log.1-4          ← Rotações antigas
└── monitoring\            ← Stack de monitoramento (opcional)
    ├── docker-compose.yml
    ├── README.md
    ├── prometheus\
    │   └── prometheus.yml
    └── grafana\
        ├── provisioning\
        └── dashboards\
```

---

## ✅ Checklist de Deploy

**Obrigatório**:
- [ ] FFmpeg instalado (via setup.ps1 ou manual)
- [ ] NSSM baixado
- [ ] Diretório C:\EdgeVideo criado
- [ ] producer.exe + config.yaml copiados
- [ ] config.yaml configurado (senhas, URLs)
- [ ] Teste manual bem-sucedido
- [ ] Serviço instalado com NSSM
- [ ] Auto-restart configurado
- [ ] Logs configurados (rotação)
- [ ] Serviço iniciado
- [ ] Métricas acessíveis (localhost:2112)
- [ ] Logs monitorados (sem erros)
- [ ] Reboot testado (serviço volta automaticamente)

**Opcional (Monitoramento)**:
- [ ] Docker Desktop instalado
- [ ] pasta monitoring/ copiada
- [ ] docker-compose up -d executado
- [ ] Grafana acessível (localhost:3000)
- [ ] Dashboard "Edge Video V2" configurado

---

## 🎯 Próximos Passos

1. **Alertas**: Configure Prometheus Alertmanager para notificações (email/Slack/Discord)
2. **Backup**: Agende backup diário do config.yaml
3. **HA**: Para alta disponibilidade, rode múltiplas instâncias em máquinas diferentes
4. **Segurança**: Configure firewall Windows para expor apenas portas necessárias

---

**Suporte**: Issues no GitHub com logs (stdout.log + stderr.log)
**Métricas**: http://localhost:2112/metrics
**pprof**: http://localhost:6060/debug/pprof/
