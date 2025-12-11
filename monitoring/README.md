# 📊 Monitoramento Edge Video V2

Sistema completo de monitoramento em tempo real para Edge Video V2, com métricas Prometheus e dashboard HTML standalone.

---

## 🚀 Início Rápido

### 1. Iniciar Edge Video V2

```bash
cd v2
./edge-video-v2.exe
```

O servidor de métricas Prometheus estará disponível em:
- **Métricas**: http://localhost:2112/metrics
- **Health Check**: http://localhost:2112/health
- **pprof (debug)**: http://localhost:6060/debug/pprof/

### 2. Visualizar Dashboard

Abra o arquivo `monitoring/dashboard.html` diretamente no seu navegador:

```
D:\Users\rafa2\OneDrive\Desktop\edge-video\v2\monitoring\dashboard.html
```

O dashboard atualiza automaticamente a cada 5 segundos, exibindo:
- ✅ Estatísticas globais do sistema
- 🎥 Status individual de cada câmera
- 📈 Métricas em tempo real
- 🔴 Alertas de circuit breakers OPEN

---

## 📊 Métricas Disponíveis

### **Métricas por Câmera**

| Métrica | Tipo | Descrição |
|---------|------|-----------|
| `edge_video_frames_received_total` | Counter | Total de frames recebidos do FFmpeg |
| `edge_video_frames_published_total` | Counter | Total de frames publicados no RabbitMQ |
| `edge_video_frames_dropped_total` | Counter | Total de frames descartados (buffer cheio) |
| `edge_video_publish_errors_total` | Counter | Total de erros ao publicar |
| `edge_video_camera_fps` | Gauge | FPS real da câmera |
| `edge_video_publish_latency_ms` | Gauge | Latência de publicação em ms |
| `edge_video_frame_size_bytes` | Gauge | Tamanho do último frame |
| `edge_video_circuit_breaker_state` | Gauge | Estado do circuit breaker (0=CLOSED, 1=OPEN, 2=HALF_OPEN) |
| `edge_video_publish_duration_seconds` | Histogram | Distribuição de tempos de publicação |
| `edge_video_frame_size_bytes_histogram` | Histogram | Distribuição de tamanhos de frames |

### **Métricas Globais do Sistema**

| Métrica | Tipo | Descrição |
|---------|------|-----------|
| `edge_video_publisher_confirms_ack_total` | Counter | Total de ACKs do RabbitMQ |
| `edge_video_publisher_confirms_nack_total` | Counter | Total de NACKs do RabbitMQ |
| `edge_video_system_cpu_percent` | Gauge | Uso de CPU do processo (%) |
| `edge_video_system_ram_mb` | Gauge | Uso de RAM em MB |
| `edge_video_system_goroutines` | Gauge | Número de goroutines ativas |
| `edge_video_system_gc_total` | Counter | Número total de GC executados |
| `edge_video_circuit_breakers_open` | Gauge | Número de circuit breakers OPEN |
| `edge_video_uptime_seconds` | Gauge | Tempo de execução em segundos |

---

## 🎯 Exemplos de Uso

### Ver Métricas Brutas

```bash
curl http://localhost:2112/metrics
```

### Filtrar Métricas Específicas

**Frames recebidos por câmera:**
```bash
curl -s http://localhost:2112/metrics | grep "frames_received_total"
```

**Status dos circuit breakers:**
```bash
curl -s http://localhost:2112/metrics | grep "circuit_breaker_state"
```

**Uso de recursos do sistema:**
```bash
curl -s http://localhost:2112/metrics | grep -E "system_|uptime"
```

---

## 📈 Dashboard HTML

### Recursos do Dashboard

- **Auto-atualização**: Recarrega dados a cada 5 segundos
- **Sem dependências externas**: Roda 100% no navegador
- **Visual moderno**: Design glassmorphism com gradientes
- **Responsivo**: Adapta-se a diferentes tamanhos de tela

### Seções do Dashboard

1. **Estatísticas Globais** (6 cards):
   - Total de Frames Publicados
   - Taxa de Confirmação (% ACK)
   - Uso de RAM
   - Goroutines Ativas
   - Tempo de Execução
   - Circuit Breakers OPEN

2. **Status das Câmeras** (grid):
   - Status (ONLINE/OFFLINE/CB OPEN)
   - Frames Recebidos/Publicados/Descartados
   - FPS Real
   - Latência de Publicação
   - Tamanho do Frame

### Indicadores de Status

- 🟢 **ONLINE**: Câmera funcionando normalmente
- 🟡 **CB OPEN**: Circuit breaker aberto (reconectando)
- 🔴 **OFFLINE**: Câmera parada (0 frames recebidos)

---

## 🐳 Opção: Usar Prometheus + Grafana com Docker

Se você quiser usar o stack completo Prometheus + Grafana (opcional):

### 1. Iniciar Stack

```bash
cd monitoring
docker-compose up -d
```

### 2. Acessar Interfaces

- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (admin/admin)

### 3. Dashboard Grafana

O dashboard está pré-configurado em:
```
monitoring/grafana/dashboards/edge-video-v2-dashboard.json
```

Acesse Grafana → Dashboards → "Edge Video V2"

### 4. Parar Stack

```bash
docker-compose down
```

---

## 🔍 Troubleshooting

### Dashboard não carrega dados

**Problema**: "Erro ao conectar com o servidor de métricas"

**Solução**:
1. Verifique se Edge Video V2 está rodando
2. Teste o endpoint: `curl http://localhost:2112/metrics`
3. Verifique CORS se estiver usando file:// (use servidor HTTP local)

### Servir dashboard com servidor HTTP local

```bash
# Opção 1: Python
cd monitoring
python -m http.server 8080

# Opção 2: Node.js
npx http-server monitoring -p 8080
```

Acesse: http://localhost:8080/dashboard.html

### Métricas não aparecem

1. **Verifique logs do Edge Video V2**:
   ```
   📊 Prometheus metrics server rodando em http://localhost:2112/metrics
   📊 Métricas registradas para câmera: cam1
   ```

2. **Teste health check**:
   ```bash
   curl http://localhost:2112/health
   # Deve retornar: OK
   ```

3. **Verifique métricas específicas**:
   ```bash
   curl http://localhost:2112/metrics | grep edge_video
   ```

---

## 🎓 Entendendo as Métricas

### Taxa de ACK = 100%
✅ **Excelente!** Todas as mensagens foram confirmadas pelo RabbitMQ.

### Frames Dropped > 0
⚠️ **Atenção**: Buffer de processamento está cheio. Possíveis causas:
- RabbitMQ lento
- Rede saturada
- Latência alta

**Solução**: Ajustar `prefetch_count` no `config.yml`

### Circuit Breaker OPEN
🔴 **Crítico**: Câmera com falhas consecutivas, aguardando backoff.

**Verifique**:
1. Câmera está acessível?
2. Credenciais corretas?
3. Rede estável?

### RAM crescendo constantemente
🚨 **Memory Leak?**

**Diagnóstico**:
1. Acesse pprof: http://localhost:6060/debug/pprof/heap
2. Verifique goroutines: http://localhost:6060/debug/pprof/goroutine?debug=1

### Goroutines > 100
⚠️ **Possível problema**: Número esperado = `10 + (6 câmeras × 2) = 22 goroutines`

**Investigar**:
```bash
curl http://localhost:6060/debug/pprof/goroutine?debug=1
```

---

## 📊 Queries Prometheus Úteis

Se estiver usando Prometheus, queries úteis para alertas:

### Taxa de Sucesso
```promql
rate(edge_video_publisher_confirms_ack_total[5m])
/
(rate(edge_video_publisher_confirms_ack_total[5m]) + rate(edge_video_publisher_confirms_nack_total[5m]))
```

### FPS por Câmera
```promql
edge_video_camera_fps{camera_id="cam1"}
```

### Latência Média de Publicação
```promql
avg(edge_video_publish_latency_ms)
```

### Frames Descartados (rate)
```promql
rate(edge_video_frames_dropped_total[1m])
```

### Circuit Breakers OPEN
```promql
count(edge_video_circuit_breaker_state == 1)
```

---

## 🎯 Próximos Passos

1. ✅ Métricas Prometheus funcionando
2. ✅ Dashboard HTML standalone
3. ⏳ **Configurar alertas** (Prometheus Alertmanager)
4. ⏳ **Integração com Slack/Discord** para notificações
5. ⏳ **Exportar métricas para InfluxDB** (optional)

---

## 📝 Arquivos de Configuração

```
v2/monitoring/
├── dashboard.html                           # Dashboard HTML standalone
├── README.md                                # Esta documentação
├── docker-compose.yml                       # Stack Prometheus + Grafana (opcional)
├── prometheus/
│   └── prometheus.yml                       # Config do Prometheus
└── grafana/
    ├── provisioning/
    │   ├── datasources/prometheus.yml       # Auto-provisioning datasource
    │   └── dashboards/dashboard.yml         # Auto-provisioning dashboard
    └── dashboards/
        └── edge-video-v2-dashboard.json     # Dashboard Grafana profissional
```

---

## 🏆 Métricas de Produção Esperadas

Sistema funcionando perfeitamente:

```
✅ ACK Rate: 100%
✅ NACK Count: 0
✅ RAM: ~200-250 MB (estável)
✅ Goroutines: 20-25 (estável)
✅ Frames Dropped: 0 ou muito baixo (<1%)
✅ Circuit Breakers: 0 OPEN
✅ Latência: <10ms média
```

---

**Desenvolvido com ❤️ para Edge Video V2**
