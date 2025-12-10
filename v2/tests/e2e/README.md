# 🚀 End-to-End Tests - Fase 3

## Objetivo
Validar o sistema Edge Video V2 completo com câmeras reais em ambiente de staging/produção.

## Pré-requisitos

### Serviços Externos (Obrigatórios)
- ✅ Redis: `34.30.59.236:6379` (senha: `MinhaSenhaMuitoForte@2024`)
- ✅ RabbitMQ: `34.71.212.239:5672` (vhost: `supercarlao_rj_mercado`)

### Câmeras Configuradas
1. **cam1** (RTMP): Mercado Autônomo - `rtmp://str-rtmp-001.monuv.com.br:1935/rtmp/96159.stream`
2. **cam2** (RTSP): Pix Force Canal 1 - `rtsp://pixforce:pixforce1234@186.193.228.105:12554/cam/realmonitor?channel=1&subtype=0`
3. **cam3** (RTSP): Pix Force Canal 2 - `rtsp://pixforce:pixforce1234@186.193.228.105:12554/cam/realmonitor?channel=2&subtype=0`
4. **cam4** (RTSP): Pix Force Canal 3 - `rtsp://pixforce:pixforce1234@186.193.228.105:12554/cam/realmonitor?channel=3&subtype=0`
5. **cam5** (RTSP): Pix Force Canal 4 - `rtsp://pixforce:pixforce1234@186.193.228.105:12554/cam/realmonitor?channel=4&subtype=0`

## Testes Implementados

### 1. Environment Setup Validation (`setup_test.go`)
- ✅ Validar conectividade com Redis
- ✅ Validar conectividade com RabbitMQ
- ✅ Validar config.yaml
- ✅ Validar binário producer compilado

### 2. Single Camera Tests (`single_camera_test.go`)
- ✅ Testar cam1 (RTMP) isoladamente
- ✅ Testar cam2 (RTSP) isoladamente
- ✅ Validar frames no Redis
- ✅ Validar metadata no RabbitMQ
- ✅ Validar Health Monitor stats

### 3. Multi-Camera Stress Test (`stress_test.go`)
- ✅ Rodar 5 câmeras simultâneas @ 15 FPS
- ✅ Monitorar throughput sustentado
- ✅ Monitorar CPU usage
- ✅ Monitorar Memory usage
- ✅ Validar frame drop rate <2%
- ✅ Duração: 5 minutos contínuos

### 4. Reconnection Tests (`reconnection_test.go`)
- ✅ Simular queda de câmera
- ✅ Validar Circuit Breaker abrindo
- ✅ Validar reconexão automática
- ✅ Validar Circuit Breaker fechando

### 5. Graceful Shutdown Test (`shutdown_test.go`)
- ✅ Iniciar 5 câmeras
- ✅ Enviar SIGTERM/SIGINT
- ✅ Validar shutdown <5s
- ✅ Validar último frame publicado

### 6. Long Running Soak Test (`soak_test.go`)
- ✅ Rodar sistema por 1 hora
- ✅ Monitorar memory leaks
- ✅ Monitorar goroutine leaks
- ✅ Validar estabilidade

## Como Executar

### Setup Inicial
```bash
# Build do producer
cd v2
go build -o bin/producer.exe ./cmd/producer

# Validar ambiente
go test -v ./tests/e2e -run TestEnvironmentSetup
```

### Testes Individuais
```bash
# Single camera
go test -v ./tests/e2e -run TestSingleCamera -timeout 5m

# Stress test (5 câmeras)
go test -v ./tests/e2e -run TestStress -timeout 10m

# Reconnection
go test -v ./tests/e2e -run TestReconnection -timeout 5m

# Graceful shutdown
go test -v ./tests/e2e -run TestGracefulShutdown -timeout 2m
```

### Suite Completa
```bash
# Rodar todos os testes E2E (exceto soak test)
go test -v ./tests/e2e -timeout 30m

# Incluir soak test (1h+)
go test -v ./tests/e2e -run TestSoak -timeout 2h
```

### Modo Automatizado
```bash
# Script que roda tudo e gera relatório
./tests/e2e/run_e2e.sh
```

## Critérios de Sucesso

| Critério | Target | Status |
|----------|--------|--------|
| 5 câmeras simultâneas | Sem crashes | ⏳ |
| Frame drop rate | <2% | ⏳ |
| Memory usage | Estável (<500MB) | ⏳ |
| CPU usage | <30% | ⏳ |
| Reconexão automática | Funcional | ⏳ |
| Shutdown graceful | <5s | ⏳ |
| Soak test 1h | Sem degradação | ⏳ |

## Estrutura de Arquivos

```
tests/e2e/
├── README.md                    # Este arquivo
├── setup_test.go               # Validação de ambiente
├── single_camera_test.go       # Testes com 1 câmera
├── stress_test.go              # Stress com 5 câmeras
├── reconnection_test.go        # Testes de reconexão
├── shutdown_test.go            # Graceful shutdown
├── soak_test.go                # Long-running test
├── helpers.go                  # Funções auxiliares
├── run_e2e.sh                  # Script de execução
└── RESULTS.md                  # Resultados dos testes
```

## Notas Importantes

⚠️ **Atenção**: Estes testes consomem recursos reais:
- Conectam em câmeras reais
- Armazenam frames no Redis de produção
- Publicam no RabbitMQ de produção

🔒 **Segurança**: Nunca commitar senhas no repositório. Use variáveis de ambiente em produção.

📊 **Monitoramento**: Durante os testes, monitore:
- `htop` ou `top` para CPU/Memory
- Redis: `redis-cli monitor` (em outro terminal)
- RabbitMQ Management UI: http://34.71.212.239:15672

## Troubleshooting

### Erro: "connection refused" no Redis
```bash
# Testar conectividade
redis-cli -h 34.30.59.236 -p 6379 -a "MinhaSenhaMuitoForte@2024" PING
```

### Erro: "connection refused" no RabbitMQ
```bash
# Testar conectividade
telnet 34.71.212.239 5672
```

### Erro: "camera stream timeout"
- Verificar se câmera está online
- Testar URL com ffplay: `ffplay rtmp://...`
- Verificar firewall/network
