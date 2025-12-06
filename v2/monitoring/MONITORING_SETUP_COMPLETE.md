# Edge Video V2 - Monitoring

## Quick Start

```powershell
cd v2/monitoring
docker-compose up -d
.\create-full-dashboard.ps1
```

**Grafana**: http://localhost:3000 (admin/admin)
**Dashboard**: http://localhost:3000/d/edge-video-v2

## Stack
- Prometheus :9090 (scrape 5s, retention 30d)
- Grafana :3000 (refresh 5s)
- Edge Video :2112/metrics

## Dashboard (12 painéis)
- System Overview: Frames, ACK Rate, RAM, Goroutines, Uptime, Circuit Breakers
- Camera Performance: Frames/Camera, Latency, FPS
- Resources: Memory, Goroutines, ACK/NACK trends
