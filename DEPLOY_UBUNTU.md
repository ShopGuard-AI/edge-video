# Edge Video V2 - Deploy Ubuntu (Systemd Service)

Guia completo para instalar Edge Video V2 no Ubuntu como serviço systemd resiliente, com auto-restart e recuperação automática.

---

## 🚀 Instalação Rápida

### 1. Download do Release

```bash
# Baixar release (substitua v1.6 pela versão desejada)
wget https://github.com/ShopGuard-AI/edge-video/releases/download/v1.6/edge-video-v1.6-linux.tar.gz

# Ou com curl
curl -LO https://github.com/ShopGuard-AI/edge-video/releases/download/v1.6/edge-video-v1.6-linux.tar.gz
```

### 2. Extrair e Rodar Setup

```bash
# Extrair
mkdir -p /opt/edge-video
tar -xzf edge-video-v1.6-linux.tar.gz -C /opt/edge-video
cd /opt/edge-video

# Rodar setup (instala FFmpeg)
chmod +x setup-ubuntu.sh
./setup-ubuntu.sh
```

O script faz automaticamente:
- ✅ Detecta distro (Ubuntu/Debian/CentOS/Arch)
- ✅ Instala FFmpeg via package manager
- ✅ Configura permissões
- ✅ Valida instalação

### 3. Configurar e Testar

```bash
# Editar config
nano config.yaml

# Testar manualmente
./producer-linux
```

**Se funcionou**, vá para seção "Instalar como Serviço Systemd" abaixo.

---

## 📋 Instalação Manual (Alternativa)

### 1. Instalar FFmpeg

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install -y ffmpeg

# CentOS/RHEL
sudo yum install -y ffmpeg

# Arch Linux
sudo pacman -S ffmpeg
```

Verificar:
```bash
ffmpeg -version
```

### 2. Instalar Producer

```bash
# Criar diretório
sudo mkdir -p /opt/edge-video
cd /opt/edge-video

# Copiar arquivos (producer-linux + config.yaml)
sudo cp /path/to/producer-linux .
sudo cp /path/to/config.yaml .

# Permissões
sudo chmod +x producer-linux
```

---

## 🔧 Instalar como Serviço Systemd

### 1. Criar Usuário (Segurança)

```bash
# Criar usuário dedicado (sem shell)
sudo useradd -r -s /bin/false -d /opt/edge-video edgevideo

# Dar ownership dos arquivos
sudo chown -R edgevideo:edgevideo /opt/edge-video
```

### 2. Criar Service File

```bash
sudo nano /etc/systemd/system/edge-video.service
```

**Conteúdo**:
```ini
[Unit]
Description=Edge Video Producer
Documentation=https://github.com/ShopGuard-AI/edge-video
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=edgevideo
Group=edgevideo
WorkingDirectory=/opt/edge-video
ExecStart=/opt/edge-video/producer-linux
Restart=always
RestartSec=5s

# Limites de recursos
LimitNOFILE=65536
MemoryMax=2G

# Logs (journald)
StandardOutput=journal
StandardError=journal
SyslogIdentifier=edge-video

# Proteções de segurança
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/edge-video

[Install]
WantedBy=multi-user.target
```

### 3. Habilitar e Iniciar Serviço

```bash
# Recarregar systemd
sudo systemctl daemon-reload

# Habilitar (inicia no boot)
sudo systemctl enable edge-video

# Iniciar serviço
sudo systemctl start edge-video

# Verificar status
sudo systemctl status edge-video
```

**Output esperado**:
```
● edge-video.service - Edge Video Producer
   Loaded: loaded (/etc/systemd/system/edge-video.service; enabled)
   Active: active (running) since...
```

---

## 🛡️ Resiliência Garantida

### Recuperação Automática

✅ **Queda de rede**: Reconexão automática (Redis + RabbitMQ)
✅ **Redis offline**: Retry com exponential backoff
✅ **RabbitMQ offline**: Reconexão automática
✅ **Câmera offline**: Circuit Breaker (30s retry)
✅ **FFmpeg crash**: Restart automático
✅ **Producer crash**: Systemd reinicia em 5s
✅ **Reboot servidor**: Serviço inicia automaticamente
✅ **Memory leak**: Memory Controller força GC

### Circuit Breaker

Protege contra câmeras com falhas persistentes:
- 5 falhas → OPEN (para tentativas)
- 30s → HALF_OPEN (testa reconexão)
- 3 sucessos → CLOSED (volta normal)
- Backoff: 5s → 5min (exponencial)

### Logs Centralizados (journald)

Systemd gerencia logs automaticamente:
- Rotação automática
- Compressão
- Retenção configurável

```bash
# Ver logs em tempo real
sudo journalctl -u edge-video -f

# Ver últimas 100 linhas
sudo journalctl -u edge-video -n 100

# Ver logs de hoje
sudo journalctl -u edge-video --since today

# Ver logs entre datas
sudo journalctl -u edge-video --since "2024-12-10" --until "2024-12-11"
```

---

## 📊 Monitoramento

### Verificar Status

```bash
# Status do serviço
sudo systemctl status edge-video

# Ver se está habilitado (boot)
sudo systemctl is-enabled edge-video

# Ver se está rodando
sudo systemctl is-active edge-video
```

### Métricas Prometheus

```bash
# Acessar métricas
curl http://localhost:2112/metrics

# FPS por câmera
curl -s http://localhost:2112/metrics | grep edge_video_camera_fps

# Memory usage
curl -s http://localhost:2112/metrics | grep edge_video_system_memory_mb

# Circuit breaker states
curl -s http://localhost:2112/metrics | grep edge_video_circuit_breaker_state
```

### Integrar com Prometheus

Editar `/etc/prometheus/prometheus.yml`:
```yaml
scrape_configs:
  - job_name: 'edge-video'
    static_configs:
      - targets: ['localhost:2112']
```

```bash
sudo systemctl restart prometheus
```

---

## 🔧 Gerenciamento do Serviço

### Comandos Básicos

```bash
# Iniciar
sudo systemctl start edge-video

# Parar
sudo systemctl stop edge-video

# Restart
sudo systemctl restart edge-video

# Status
sudo systemctl status edge-video

# Logs
sudo journalctl -u edge-video -f

# Desabilitar (não inicia no boot)
sudo systemctl disable edge-video

# Habilitar
sudo systemctl enable edge-video
```

### Atualizar Producer

```bash
# 1. Parar serviço
sudo systemctl stop edge-video

# 2. Backup
sudo cp /opt/edge-video/producer-linux /opt/edge-video/producer-linux.bak

# 3. Substituir binário
sudo cp /path/to/new/producer-linux /opt/edge-video/

# 4. Permissões
sudo chown edgevideo:edgevideo /opt/edge-video/producer-linux
sudo chmod +x /opt/edge-video/producer-linux

# 5. Restart
sudo systemctl start edge-video

# 6. Verificar logs
sudo journalctl -u edge-video -f
```

### Rollback

```bash
sudo systemctl stop edge-video
sudo cp /opt/edge-video/producer-linux.bak /opt/edge-video/producer-linux
sudo systemctl start edge-video
```

---

## 🚨 Troubleshooting

### Serviço não inicia

```bash
# Ver erro detalhado
sudo journalctl -u edge-video -n 50 --no-pager

# Testar manualmente
cd /opt/edge-video
sudo -u edgevideo ./producer-linux
```

**Causas comuns**:
- FFmpeg não instalado → `sudo apt install ffmpeg`
- config.yaml inválido → Validar YAML syntax
- Permissões incorretas → `sudo chown -R edgevideo:edgevideo /opt/edge-video`
- Redis/RabbitMQ inacessíveis → Verificar conectividade

### FPS baixo

**Causa**: Redis remoto alta latência
**Solução**: Use Redis local ou aumente TTL

### Memory > 2 GB

**Causa**: Muitas câmeras ou quality alta
**Solução**: Reduza quality (5→3) ou max_memory_mb no config.yaml

### Circuit Breaker OPEN

**Causa**: Câmera offline ou URL inválida
**Solução**:
```bash
# Teste URL com FFmpeg
ffmpeg -i "rtmp://servidor:1935/stream" -frames:v 1 test.jpg
```

### Logs não aparecem

```bash
# Verificar se serviço está rodando
sudo systemctl status edge-video

# Ver todos os logs (sem filtro)
sudo journalctl -u edge-video --no-pager
```

---

## 📁 Estrutura de Diretórios

```
/opt/edge-video/
├── producer-linux         ← Executável
├── config.yaml            ← Configuração
├── producer-linux.bak     ← Backup (atualizações)
└── logs/                  ← Logs locais (se habilitado)
```

**Logs systemd**:
```
/var/log/journal/          ← Logs gerenciados pelo journald
```

---

## 🔒 Segurança

### Firewall (UFW)

```bash
# Permitir Prometheus (se externo)
sudo ufw allow 2112/tcp comment 'Edge Video Metrics'

# Verificar
sudo ufw status
```

### Hardening Adicional

No service file (`/etc/systemd/system/edge-video.service`), adicione:

```ini
[Service]
# ... (outras configurações)

# Proteções adicionais
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictRealtime=true
RestrictNamespaces=true
```

```bash
sudo systemctl daemon-reload
sudo systemctl restart edge-video
```

---

## ✅ Checklist de Deploy

- [ ] FFmpeg instalado (`ffmpeg -version`)
- [ ] Usuário `edgevideo` criado
- [ ] Diretório `/opt/edge-video` criado
- [ ] `producer-linux` copiado e executável
- [ ] `config.yaml` configurado (senhas, URLs)
- [ ] Service file criado (`/etc/systemd/system/edge-video.service`)
- [ ] Systemd recarregado (`daemon-reload`)
- [ ] Serviço habilitado (`enable`)
- [ ] Serviço iniciado (`start`)
- [ ] Status verificado (active/running)
- [ ] Logs monitorados (sem erros)
- [ ] Métricas acessíveis (`curl localhost:2112/metrics`)
- [ ] Reboot testado (serviço volta automaticamente)

---

## 🎯 Próximos Passos

1. **Monitoramento**: Configure Grafana + Prometheus (ver `monitoring/`)
2. **Backup**: Agende backup diário do config.yaml
3. **Alertas**: Configure notificações (email/Slack) para métricas críticas
4. **HA**: Para alta disponibilidade, rode múltiplas instâncias (1 por câmera)

---

**Suporte**: Issues no GitHub com logs (`journalctl -u edge-video`)
**Métricas**: http://localhost:2112/metrics
**Logs**: `sudo journalctl -u edge-video -f`
