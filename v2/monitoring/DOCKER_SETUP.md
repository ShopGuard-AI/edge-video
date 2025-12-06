# 🐳 Setup Docker - Prometheus + Grafana

## Arquitetura

```
┌──────────────────────────────────────┐
│  WINDOWS (seu PC)                    │
│                                      │
│  ┌────────────────────┐              │
│  │  Edge Video V2     │              │
│  │  (executável .exe) │              │
│  │                    │              │
│  │  Port 2112 ────────┼──┐           │
│  └────────────────────┘  │           │
│                          │           │
│  ┌───────────────────────┼──────────┐│
│  │  DOCKER CONTAINERS    ↓          ││
│  │  ┌─────────────────────────────┐ ││
│  │  │ Prometheus                  │ ││
│  │  │ - Coleta métricas do host   │ ││
│  │  │ - Guarda histórico 30 dias  │ ││
│  │  │ - Port 9090                 │ ││
│  │  └─────────────┬───────────────┘ ││
│  │                ↓                 ││
│  │  ┌─────────────────────────────┐ ││
│  │  │ Grafana                     │ ││
│  │  │ - Dashboards visuais        │ ││
│  │  │ - Alertas                   │ ││
│  │  │ - Port 3000                 │ ││
│  │  └─────────────────────────────┘ ││
│  └─────────────────────────────────┘│
└──────────────────────────────────────┘
```

---

## 📋 Pré-requisitos

### 1. Instalar Docker Desktop

Se você ainda não tem Docker Desktop instalado:

1. **Download**: https://www.docker.com/products/docker-desktop/
2. **Instalar** e reiniciar o PC se necessário
3. **Abrir Docker Desktop** e aguardar ele iniciar completamente

### 2. Verificar Docker

```powershell
docker --version
docker-compose --version
```

Deve mostrar algo como:
```
Docker version 24.x.x
Docker Compose version v2.x.x
```

---

## 🚀 Iniciar Prometheus + Grafana

### 1. Certifique-se que Edge Video está rodando

```powershell
# O Edge Video deve estar rodando ANTES de iniciar o Docker
# Teste se as métricas estão disponíveis:
curl http://localhost:2112/metrics
```

Se não retornar métricas, inicie o Edge Video:
```powershell
cd D:\Users\rafa2\OneDrive\Desktop\edge-video\v2
.\edge-video-v2.exe
```

### 2. Iniciar Docker Stack

```powershell
cd D:\Users\rafa2\OneDrive\Desktop\edge-video\v2\monitoring
docker-compose up -d
```

Saída esperada:
```
Creating network "monitoring_monitoring" with driver "bridge"
Creating volume "monitoring_prometheus-data" with default driver
Creating volume "monitoring_grafana-data" with default driver
Creating edge-video-prometheus ... done
Creating edge-video-grafana     ... done
```

### 3. Verificar containers rodando

```powershell
docker-compose ps
```

Deve mostrar:
```
NAME                    STATUS              PORTS
edge-video-prometheus   Up X seconds        0.0.0.0:9090->9090/tcp
edge-video-grafana      Up X seconds        0.0.0.0:3000->3000/tcp
```

---

## 🌐 Acessar Interfaces

### Prometheus
- **URL**: http://localhost:9090
- **Uso**: Queries PromQL, verificar targets, alertas

### Grafana
- **URL**: http://localhost:3000
- **Login**: `admin` / `admin`
- **Primeiro acesso**: Vai pedir para trocar a senha (você pode pular)

---

## 📊 Configurar Grafana

### Dashboard já está pré-configurado!

O dashboard "Edge Video V2" já foi provisionado automaticamente. Para acessar:

1. Abra http://localhost:3000
2. Login: `admin` / `admin`
3. No menu lateral: **Dashboards** → **Edge Video V2**

### Se o dashboard não aparecer:

1. Vá em **Dashboards** → **New** → **Import**
2. Clique em **Upload JSON file**
3. Selecione: `D:\Users\rafa2\OneDrive\Desktop\edge-video\v2\monitoring\grafana\dashboards\edge-video-v2-dashboard.json`
4. Clique em **Load** → **Import**

---

## 🔍 Verificar se Prometheus está coletando métricas

### Opção 1: Via Prometheus UI

1. Abra http://localhost:9090
2. Vá em **Status** → **Targets**
3. Procure por `edge-video-v2`
4. Status deve estar **UP** (verde)

### Opção 2: Via linha de comando

```powershell
curl http://localhost:9090/api/v1/targets
```

Procure por:
```json
{
  "labels": {
    "job": "edge-video-v2"
  },
  "health": "up"
}
```

---

## 🛑 Parar Docker Stack

```powershell
cd D:\Users\rafa2\OneDrive\Desktop\edge-video\v2\monitoring
docker-compose down
```

**IMPORTANTE**: Isso para os containers mas **NÃO apaga os dados**. Os dados ficam salvos nos volumes `prometheus-data` e `grafana-data`.

---

## 🗑️ Parar e LIMPAR tudo (apaga histórico)

Se quiser começar do zero:

```powershell
docker-compose down -v
```

O `-v` remove os volumes (histórico de métricas, configurações Grafana, etc.)

---

## 🔧 Troubleshooting

### Problema: Target "edge-video-v2" aparece como DOWN

**Causa**: Prometheus não consegue acessar o Edge Video no host

**Solução**:

1. Verifique se Edge Video está rodando:
   ```powershell
   curl http://localhost:2112/metrics
   ```

2. No Docker Desktop:
   - Vá em **Settings** → **Resources** → **Network**
   - Certifique-se que "Use host networking" está habilitado (se disponível)

3. Se estiver no Linux, edite `prometheus/prometheus.yml`:
   ```yaml
   targets: ['172.17.0.1:2112']  # IP padrão do Docker bridge
   ```

### Problema: Grafana não mostra dados

1. **Verifique Prometheus**:
   - Abra http://localhost:9090
   - Execute query: `edge_video_frames_received_total`
   - Deve retornar dados

2. **Verifique datasource**:
   - Grafana → **Connections** → **Data sources** → **Prometheus**
   - URL deve ser: `http://prometheus:9090`
   - Clique em **Save & test** → deve aparecer "Data source is working"

3. **Verifique range de tempo**:
   - No dashboard, certifique-se que o range está em "Last 15 minutes" ou "Last 1 hour"

### Problema: Docker não inicia

```powershell
# Ver logs
docker-compose logs prometheus
docker-compose logs grafana

# Reiniciar containers
docker-compose restart
```

---

## 📈 Queries úteis no Prometheus

Acesse http://localhost:9090/graph e teste:

### Taxa de frames por segundo
```promql
rate(edge_video_frames_published_total[1m])
```

### Taxa de sucesso (ACK rate)
```promql
rate(edge_video_publisher_confirms_ack_total[5m])
/
(rate(edge_video_publisher_confirms_ack_total[5m]) + rate(edge_video_publisher_confirms_nack_total[5m]))
```

### Uso de RAM
```promql
edge_video_system_ram_mb
```

### Frames descartados (rate)
```promql
rate(edge_video_frames_dropped_total[1m])
```

### Circuit Breakers OPEN
```promql
count(edge_video_circuit_breaker_state == 1)
```

---

## 🎯 Resumo dos Comandos

```powershell
# INICIAR stack
cd D:\Users\rafa2\OneDrive\Desktop\edge-video\v2\monitoring
docker-compose up -d

# VER status
docker-compose ps

# VER logs em tempo real
docker-compose logs -f

# PARAR (mantém dados)
docker-compose down

# PARAR E LIMPAR TUDO (apaga dados)
docker-compose down -v

# REINICIAR apenas um serviço
docker-compose restart prometheus
docker-compose restart grafana
```

---

## 📂 Persistência de Dados

Os dados são salvos em volumes Docker:

- **prometheus-data**: Histórico de métricas (30 dias configurados)
- **grafana-data**: Dashboards, usuários, configurações

Mesmo se você rodar `docker-compose down`, os dados **NÃO são apagados**. Só serão removidos com `docker-compose down -v`.

---

## 🏆 Workflow Recomendado

### Desenvolvimento diário:
1. **Iniciar Edge Video** (executável)
2. **Abrir dashboard.html** no navegador (para visualização rápida)

### Análise aprofundada:
1. **Iniciar Edge Video** (executável)
2. **Iniciar Docker stack** (`docker-compose up -d`)
3. **Abrir Grafana** (http://localhost:3000) para análise profissional

### Antes de desligar o PC:
```powershell
# Parar Edge Video (Ctrl+C)
# Parar Docker (opcional - pode deixar rodando)
docker-compose down
```

---

**Pronto!** Agora você tem:
- ✅ Edge Video rodando FORA do Docker (leve e rápido)
- ✅ Prometheus + Grafana no Docker (profissional)
- ✅ Dashboard HTML standalone (sem Docker, para uso rápido)

**Melhor dos dois mundos!** 🚀
