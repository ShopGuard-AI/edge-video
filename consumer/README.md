# Edge Video Consumer

Consumer de alta performance para processar frames do Edge Video V2 via RabbitMQ.

## Quick Start

```bash
# Instalar dependências
pip install -r requirements.txt

# Rodar (já vem configurado!)
python src/consumer.py
```

Janela OpenCV abre mostrando 1 câmera com métricas em tempo real. Para parar: **Ctrl+C**

---

## Estrutura

```
consumer/
├── src/consumer.py      # Código principal
├── config.yaml          # Configuração
├── requirements.txt     # Dependências
└── README.md           # Este arquivo
```

---

## Configuração (config.yaml)

```yaml
rabbitmq:
  url: "amqp://user:pass@host:port/vhost"
  queue: "supercarlao_rj_mercado.frames"
  prefetch_count: 50      # Buffer (maior = mais throughput)
  auto_ack: false         # Manual ACK (mais seguro)

redis:
  enabled: true           # OBRIGATÓRIO (V1.6-style - body só tem redis_key)
  address: "host:6379"
  timeout: 1

processing:
  save_frames: false      # Salvar frames em disco?
  log_every_n: 100        # Log a cada N frames
  max_workers: 4

metrics:
  enabled: true
  port: 9090              # http://localhost:9090/metrics
```

---

## Métricas na Tela

**Painel Superior:**
- CAMERA: Nome (verde)
- FPS: Frames/segundo (verde >10, laranja <10)
- Frames: Total processado
- Proc: Tempo médio (ms)
- Uptime: HH:MM:SS
- Redis: Taxa de acerto (%)
- Errors: Total e %

**Painel Inferior:**
- CAMERAS: Lista com contador (verde=exibida, cinza=outras)

---

## Performance

**Testado:**
- Throughput: 84+ FPS (6 câmeras)
- Display: 22 FPS (1 câmera)
- Latência: <20ms/frame
- Taxa erro: 0%

**Otimizações:**
- Exibe apenas 1 câmera (economiza CPU/GPU)
- Outras câmeras: valida sem decodificar
- V1.6-style: Redis OBRIGATÓRIO (body só tem redis_key)
- Prefetch alto (50 frames)

---

## Troubleshooting

### FPS baixo (<10)
```bash
# 1. Aumente prefetch
prefetch_count: 100

# 2. Desabilite save_frames
save_frames: false

# 3. Verifique CPU/GPU
```

### Não conecta
```bash
# Teste RabbitMQ
rabbitmqadmin list queues

# Teste Redis (se habilitado)
redis-cli -h host -a senha PING
```

### Janela não abre (Linux)
```bash
sudo apt-get install libgl1-mesa-glx
pip uninstall opencv-python && pip install opencv-python
```

---

## Métricas Prometheus

Acesse: `http://localhost:9090/metrics`

**Queries úteis:**
```promql
# Throughput (frames/s)
rate(consumer_frames_consumed_total{status="success"}[1m])

# Latência média
rate(consumer_frames_processing_duration_seconds_sum[1m]) /
rate(consumer_frames_processing_duration_seconds_count[1m])

# Taxa de erro
rate(consumer_frames_consumed_total{status="error"}[1m]) /
rate(consumer_frames_consumed_total[1m])
```

---

## Integração Grafana

1. Edite `v2/monitoring/prometheus/prometheus.yml`:
```yaml
scrape_configs:
  - job_name: 'edge-video-consumer'
    static_configs:
      - targets: ['host.docker.internal:9090']
```

2. Restart: `cd v2/monitoring && docker-compose restart prometheus`

---

## Produção

### Systemd (Linux)
```ini
[Unit]
Description=Edge Video Consumer
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/edge-video/v2/consumer
ExecStart=/usr/bin/python3 src/consumer.py
Restart=always

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable edge-video-consumer
sudo systemctl start edge-video-consumer
```

### Docker
```dockerfile
FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
CMD ["python", "src/consumer.py"]
```

---

## Arquitetura

**Fluxo:**
1. Conecta RabbitMQ com prefetch=50
2. Seleciona primeira câmera que chegar
3. Exibe apenas essa câmera
4. Outras câmeras: valida mas não decodifica (rápido!)
5. ACK/NACK manual (thread principal - pika não é thread-safe)

**Por que síncrono?**
Pika BlockingConnection não suporta ACK/NACK de threads diferentes.

**Por que só 1 câmera?**
Decodificar 6 câmeras sobrecarrega CPU/GPU. Melhor processar todas mas exibir só 1.

---

## Logs

```
[INFO] ✓ Consumer started successfully!
[INFO] 📹 Exibindo frames da câmera: cam3
[INFO] Processed 100 frames (34.0 fps) - Success: 99, Errors: 0
[INFO] Processed 300 frames (46.8 fps) - Success: 299, Errors: 0
```

---

## FAQ

**P: Como trocar câmera exibida?**
R: Reinicie o consumer (primeira que chegar é exibida)

**P: Redis é obrigatório?**
R: SIM! Arquitetura V1.6-style requer Redis (body só tem redis_key, não o frame)

**P: Diferença FPS vs Throughput?**
R: FPS = exibidos (1 cam), Throughput = processados (todas cams)

**P: Como processar sem exibir?**
R: Remova `cv2.imshow()` e `cv2.waitKey()` do código

---

## Dependências

- `pika` - RabbitMQ
- `redis` - Redis (opcional)
- `opencv-python` - Display
- `numpy` - Arrays
- `prometheus-client` - Métricas
- `pyyaml` - Config
- `coloredlogs` - Logs

Install: `pip install -r requirements.txt`

---

**Estatísticas finais ao parar (Ctrl+C):**
```
Uptime:          98.3s
Total Consumed:  8299 frames
Success:         8299 (100.0%)
Throughput:      84.46 frames/s

Per-Camera:
  [cam1]: 1592 frames (16.2 fps)
  [cam3]: 2596 frames (26.4 fps)  ← exibida
  [cam5]: 4111 frames (41.8 fps)
```
