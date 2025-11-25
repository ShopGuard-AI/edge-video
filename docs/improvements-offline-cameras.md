# Melhorias para Cenários de Câmeras Offline

Este documento descreve as melhorias implementadas no Edge Video para lidar com cenários onde não há câmeras RTSP ativas ou quando todas as câmeras estão offline/desconectadas.

## Melhorias Implementadas

### 1. Monitor de Câmeras (Camera Monitor)

**Arquivo:** `pkg/camera/monitor.go`

Um novo componente que monitora continuamente o status de todas as câmeras no sistema.

**Funcionalidades:**
- Registra e rastreia o status de cada câmera (ativa/inativa)
- Detecta quando uma câmera fica offline após 3 falhas consecutivas
- Detecta quando não há nenhuma câmera ativa no sistema
- Registra timestamp da última captura bem-sucedida
- Conta falhas consecutivas por câmera
- Executa verificações periódicas de saúde (a cada 30 segundos por padrão)

**Callbacks configuráveis:**
- `onAllInactive`: Disparado quando todas as câmeras ficam inativas
- `onCameraDown`: Disparado quando uma câmera específica fica offline
- `onCameraUp`: Disparado quando uma câmera volta a ficar online

### 2. Novas Métricas Prometheus

**Arquivo:** `pkg/metrics/collector.go`

Métricas adicionadas para monitoramento avançado:

```go
// Timestamp Unix da última captura bem-sucedida
edge_video_last_successful_capture_timestamp{camera_id="cam1"}

// Total de tentativas de reconexão por câmera
edge_video_camera_reconnect_attempts_total{camera_id="cam1"}

// Número total de câmeras atualmente ativas
edge_video_active_cameras_total
```

**Uso:**
- Configure alertas no Grafana/Prometheus quando `edge_video_active_cameras_total` = 0
- Monitore `edge_video_last_successful_capture_timestamp` para detectar câmeras que pararam de responder
- Rastreie `edge_video_camera_reconnect_attempts_total` para identificar câmeras problemáticas

### 3. Eventos de Status no RabbitMQ

**Arquivo:** `internal/metadata/publisher.go`

Três tipos de eventos agora são publicados:

#### a) Eventos de Frame (`event_type: "frame"`)
Evento normal com metadata de frame capturado:
```json
{
  "event_type": "frame",
  "camera_id": "cam1",
  "timestamp": "2024-11-25T14:30:00Z",
  "redis_key": "vhost:frames:cam1:1731073800123456789:00001",
  "width": 1280,
  "height": 720,
  "size_bytes": 245678,
  "encoding": "jpeg"
}
```

#### b) Eventos de Status de Câmera (`event_type: "camera_status"`)
Publicado quando uma câmera muda de estado:
```json
{
  "event_type": "camera_status",
  "camera_id": "cam1",
  "timestamp": "2024-11-25T14:35:00Z",
  "state": "inactive",
  "consecutive_failures": 5,
  "last_error": "connection refused",
  "message": "Câmera tornou-se inativa após múltiplas falhas"
}
```

Estados possíveis:
- `active`: Câmera operando normalmente
- `inactive`: Câmera offline após múltiplas falhas
- `offline`: Câmera permanentemente inacessível

#### c) Eventos de Status do Sistema (`event_type: "system_status"`)
Publicado quando não há câmeras ativas:
```json
{
  "event_type": "system_status",
  "timestamp": "2024-11-25T14:40:00Z",
  "total_cameras": 5,
  "active_cameras": 0,
  "inactive_cameras": 5,
  "message": "ALERTA: Nenhuma câmera ativa detectada. Sistema em estado crítico."
}
```

**Routing Keys:**
- `camera.metadata.event` - Eventos de frames
- `camera.metadata.status` - Eventos de status de câmeras
- `camera.metadata.system` - Eventos de status do sistema

### 4. Circuit Breaker com Backoff Exponencial

**Arquivo:** `pkg/circuit/breaker.go`

O Circuit Breaker foi aprimorado com backoff exponencial para evitar sobrecarga de tentativas de reconexão.

**Características:**
- **Backoff inicial:** `resetTimeout / 2` (mínimo 5 segundos)
- **Multiplicador:** 2x a cada falha
- **Backoff máximo:** 10 minutos
- **Reset:** Volta ao backoff inicial quando a câmera reconecta com sucesso

**Exemplo de comportamento:**
```
Falha 1: Aguarda 5 segundos
Falha 2: Aguarda 10 segundos
Falha 3: Aguarda 20 segundos
Falha 4: Aguarda 40 segundos
Falha 5: Aguarda 80 segundos
Falha 6: Aguarda 160 segundos
Falha 7: Aguarda 320 segundos
Falha 8+: Aguarda 600 segundos (10 minutos - máximo)
```

**Logs melhorados:**
```
Circuit breaker cam1: CLOSED -> OPEN (falhas: 5, próxima tentativa em: 20s)
```

### 5. Logs de Diagnóstico Aprimorados

**Arquivo:** `pkg/camera/camera.go`

Os logs agora classificam erros em categorias específicas para facilitar troubleshooting:

**Tipos de erro identificados:**
- `connection_refused` - Câmera recusou conexão (IP incorreto, porta fechada)
- `connection_timeout` - Timeout na conexão (rede lenta, câmera travada)
- `auth_failed` - Credenciais incorretas (401 Unauthorized)
- `stream_not_found` - Stream RTSP não encontrado (404 Not Found)
- `context_canceled` - Aplicação encerrada durante captura
- `context_error` - Erro de contexto (timeout, cancelamento)
- `circuit_breaker_open` - Circuit breaker aberto (muitas falhas)
- `empty_frame` - Frame vazio capturado
- `ffmpeg_error` - Erro genérico do FFmpeg

**Exemplo de log detalhado:**
```
ERROR  Erro ao capturar frame
  camera_id: cam1
  error: exit status 1
  error_context: connection_refused
  stderr: Connection refused
```

### 6. Consumidor Python de Exemplo

**Arquivo:** `test_consumer_status.py`

Consumidor Python atualizado que demonstra como processar todos os tipos de eventos.

**Funcionalidades:**
- Consome eventos de frames, status de câmeras e status do sistema
- Recupera frames do Redis quando necessário
- Handlers customizáveis para cada tipo de evento
- Estatísticas de consumo
- Suporte a diferentes padrões de routing key

**Uso:**
```bash
# Instalar dependências
pip install pika redis

# Executar consumidor
python test_consumer_status.py
```

## Configuração

### Configurando Callbacks no main.go

Os callbacks são automaticamente configurados em `cmd/edge-video/main.go`:

```go
cameraMonitor := camera.NewMonitor(ctx, 30*time.Second)

if metaPublisher.Enabled() {
    cameraMonitor.SetCallbacks(
        // onAllInactive
        func(cameraID string) {
            // Publica evento de sistema crítico
        },
        // onCameraDown
        func(cameraID string) {
            // Publica evento de câmera inativa
        },
        // onCameraUp
        func(cameraID string) {
            // Publica evento de câmera ativa
        },
    )
}
```

### Configurando Backoff do Circuit Breaker

No `config.toml`:
```toml
[optimization]
circuit_max_failures = 5      # Abre o circuito após 5 falhas
circuit_reset_seconds = 60    # Backoff inicial (30s na prática)
```

O backoff inicial será `circuit_reset_seconds / 2` (mínimo 5s).

## Integração com Sistemas de Alerta

### Prometheus Alerting Rules

Exemplo de regras de alerta para Prometheus:

```yaml
groups:
  - name: edge_video_alerts
    rules:
      # Alerta quando não há câmeras ativas
      - alert: NoCamerasActive
        expr: edge_video_active_cameras_total == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Nenhuma câmera ativa detectada"
          description: "Todas as câmeras estão offline há mais de 1 minuto"

      # Alerta quando câmera não captura há muito tempo
      - alert: CameraNotResponding
        expr: (time() - edge_video_last_successful_capture_timestamp) > 300
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "Câmera {{ $labels.camera_id }} não responde"
          description: "Sem capturas há {{ $value }}s"

      # Alerta quando muitas tentativas de reconexão
      - alert: HighReconnectAttempts
        expr: rate(edge_video_camera_reconnect_attempts_total[5m]) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Câmera {{ $labels.camera_id }} instável"
          description: "Taxa de reconexão: {{ $value }} tentativas/segundo"
```

### Consumidor com Alertas

Exemplo de como integrar alertas no consumidor Python:

```python
class AlertingConsumer(EdgeVideoConsumer):
    def handle_no_active_cameras(self, total_cameras: int, message: str):
        super().handle_no_active_cameras(total_cameras, message)
        
        # Enviar alerta via webhook, email, SMS, etc.
        send_critical_alert(
            title="Sistema Edge Video Crítico",
            message=f"Nenhuma câmera ativa. Total: {total_cameras}",
            severity="critical"
        )
    
    def handle_camera_inactive(self, camera_id: str, consecutive_failures: int, last_error: str):
        super().handle_camera_inactive(camera_id, consecutive_failures, last_error)
        
        if consecutive_failures >= 10:
            send_warning_alert(
                title=f"Câmera {camera_id} offline",
                message=f"Falhas consecutivas: {consecutive_failures}. Erro: {last_error}",
                severity="warning"
            )
```

## Benefícios das Melhorias

### 1. Visibilidade
- Métricas Prometheus em tempo real do status de todas as câmeras
- Eventos RabbitMQ para sistemas downstream
- Logs detalhados com classificação de erros

### 2. Eficiência de Recursos
- Backoff exponencial reduz carga de rede e CPU em câmeras offline
- Circuit breaker evita tentativas excessivas de reconexão
- Monitor centralizado elimina verificações redundantes

### 3. Resposta Rápida
- Detecção imediata quando todas as câmeras ficam offline
- Notificações automáticas via eventos RabbitMQ
- Callbacks customizáveis para integração com sistemas de alerta

### 4. Troubleshooting
- Logs classificados por tipo de erro facilitam diagnóstico
- Métricas de reconexão identificam câmeras problemáticas
- Histórico de status de cada câmera

## Próximos Passos

### Melhorias Futuras Sugeridas

1. **Dashboard Grafana**
   - Criar dashboard específico para monitoramento de câmeras
   - Incluir gráficos de uptime, latência e taxa de falhas

2. **Reconexão Inteligente**
   - Implementar diferentes estratégias de backoff por tipo de erro
   - Aumentar backoff mais agressivamente para erros de autenticação

3. **Health Check API**
   - Expor endpoint HTTP `/health` com status de todas as câmeras
   - Incluir tempo desde última captura bem-sucedida

4. **Notificações Multi-Canal**
   - Suporte nativo para webhooks, email, Slack, Teams
   - Configuração de escalação de alertas

5. **Análise de Padrões**
   - Detectar padrões de falha (ex: sempre offline em determinado horário)
   - Sugerir ajustes de configuração baseado em histórico

## Testando as Melhorias

### Teste 1: Simular Câmera Offline

1. Configure uma câmera com URL inválida:
```toml
[[cameras]]
id = "cam_offline"
url = "rtsp://192.168.1.999:554/stream"
```

2. Inicie o sistema e observe os logs:
```bash
./edge-video --config config.toml
```

3. Verifique as métricas:
```bash
curl http://localhost:9090/metrics | grep edge_video_camera_connected
curl http://localhost:9090/metrics | grep edge_video_active_cameras_total
```

### Teste 2: Consumir Eventos de Status

1. Inicie o consumidor:
```bash
python test_consumer_status.py
```

2. Desconecte uma câmera (desligando-a ou bloqueando rede)

3. Observe os eventos publicados:
   - Evento `camera_status` com `state: "inactive"`
   - Se todas as câmeras ficarem offline, evento `system_status`

### Teste 3: Verificar Backoff Exponencial

1. Configure circuit breaker agressivo:
```toml
[optimization]
circuit_max_failures = 2
circuit_reset_seconds = 10
```

2. Observe nos logs o aumento progressivo do tempo de espera:
```
Circuit breaker cam1: CLOSED -> OPEN (falhas: 2, próxima tentativa em: 5s)
Circuit breaker cam1: HALF_OPEN -> OPEN (falhas: 3, próxima tentativa em: 10s)
Circuit breaker cam1: HALF_OPEN -> OPEN (falhas: 4, próxima tentativa em: 20s)
```

## Conclusão

As melhorias implementadas garantem que o Edge Video pode:
- Detectar e notificar quando não há câmeras ativas
- Reduzir sobrecarga de recursos em cenários de falha
- Fornecer diagnósticos detalhados para troubleshooting
- Integrar-se facilmente com sistemas de monitoramento e alerta

Para dúvidas ou sugestões, consulte a documentação completa em `docs/` ou abra uma issue no GitHub.
