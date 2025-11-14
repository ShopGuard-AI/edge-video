# Como testar o Windows CI/CD

## Pré-requisitos

Para desenvolver e testar o sistema Windows localmente, você precisa de:

### Ambiente de Desenvolvimento
- **Go 1.24+**: Para cross-compilation
- **Git**: Para versionamento e tags
- **Docker**: Para testar integração (opcional)

### Ambiente Windows (para testes)
- **Windows 7+**: Sistema alvo
- **PowerShell/CMD**: Para gerenciar serviços
- **Privilégios de Administrador**: Para instalar serviços

## Testando Localmente

### 1. Cross-compilation no Linux/macOS

```bash
# Build para Windows
./build-windows.sh

# Verificar binários gerados
ls -la dist/
# edge-video-service.exe (Windows service)
# edge-video.exe (CLI para debug)
# config/config.toml (configuração)
```

### 2. Teste em Máquina Windows

```cmd
# Copiar dist/ para Windows
scp -r dist/ user@windows-machine:C:\EdgeVideo\

# No Windows: Testar instalação manual
cd C:\EdgeVideo
edge-video-service.exe install
edge-video-service.exe start

# Verificar se está rodando
sc query EdgeVideoService
```

### 3. Teste do Instalador (requer Windows)

```cmd
# Em uma máquina Windows com NSIS instalado:
makensis /DPRODUCT_VERSION="1.2.0" installer\windows\edge-video-installer.nsi

# Executar o instalador gerado
dist\EdgeVideoSetup-1.2.0.exe
```

## Testando via GitHub Actions

### 1. Push para Branch de Teste

```bash
git checkout -b test-windows-build
git add .
git commit -m "feat: test Windows installer CI/CD"
git push origin test-windows-build

# Criar PR para main para triggerar workflow
```

### 2. Build Manual via Dispatch

```
1. GitHub → Actions → "Windows Installer Build"
2. Click "Run workflow"
3. Selecionar branch
4. Click "Run workflow"
```

### 3. Release com Tag

```bash
# Criar tag para release automático
git tag v1.2.0-test
git push origin v1.2.0-test

# GitHub Actions irá:
# 1. Buildar para Windows
# 2. Gerar instalador
# 3. Criar GitHub Release
# 4. Upload EdgeVideoSetup-1.2.0-test.exe
```

## Debuggando Problemas

### Compilação Falha

```bash
# Verificar dependências
go mod tidy
go mod verify

# Testar build local primeiro
GOOS=windows GOARCH=amd64 go build ./cmd/edge-video-service
GOOS=windows GOARCH=amd64 go build ./cmd/edge-video
```

### Service Não Instala

```cmd
# Verificar logs de instalação
eventvwr.msc → Windows Logs → System

# Testar modo console primeiro
edge-video-service.exe console

# Verificar permissões
whoami /groups | findstr Administrators
```

### Instalador NSIS Falha

```
# Verificar logs do GitHub Actions
Actions → Windows Installer Build → View logs

# Problemas comuns:
# - NSIS não instalado corretamente
# - Caminhos de arquivos incorretos
# - Variáveis de ambiente não definidas
```

## Estrutura de Teste Recomendada

### 1. Ambiente de Staging

```
Máquina Windows dedicada para testes:
- Windows 10/11 ou Server 2019+  
- Acesso via RDP
- RabbitMQ/Redis locais ou de teste
- Câmeras RTSP de teste/simuladas
```

### 2. Pipeline de Testes

```bash
# 1. Desenvolvimento local (Linux/macOS)
./build-windows.sh  # Cross-compile

# 2. Test em Windows
# - Deploy manual dos binários
# - Testar install/start/stop
# - Verificar logs

# 3. Integration test
# - Deploy via instalador NSIS
# - Configurar câmeras reais
# - Monitorar por algumas horas

# 4. Release
git tag v1.2.0
git push origin v1.2.0  # Trigger release build
```

## Monitoramento de Produção

### Event Viewer

```powershell
# Verificar logs do serviço
Get-WinEvent -LogName Application | Where-Object {$_.ProviderName -eq "EdgeVideoService"} | Select-Object -First 10

# Logs de instalação  
Get-WinEvent -LogName System | Where-Object {$_.ProviderName -eq "Service Control Manager"} | Select-Object -First 10
```

### Performance Counters

```powershell
# CPU usage
Get-Counter "\Process(edge-video-service)\% Processor Time"

# Memory usage  
Get-Counter "\Process(edge-video-service)\Working Set"

# Network connections
netstat -an | findstr :5672  # RabbitMQ
netstat -an | findstr :6379  # Redis
netstat -an | findstr :554   # RTSP
```

### Service Health Check

```cmd
# Service status
sc query EdgeVideoService

# Service config
sc qc EdgeVideoService

# Service dependencies  
sc enumdepend EdgeVideoService
```

## Troubleshooting Comum

### FFmpeg Não Encontrado
```cmd
# Verificar se FFmpeg está no PATH
where ffmpeg

# Se não estiver, adicionar ao PATH ou colocar no diretório do service
copy ffmpeg.exe "C:\Program Files\T3Labs\EdgeVideo\"
```

### Conectividade de Rede
```cmd
# Testar RabbitMQ
telnet rabbitmq-server 5672

# Testar Redis
telnet redis-server 6379

# Testar RTSP camera
ffmpeg -i "rtsp://camera-url" -t 5 -f null -
```

### Permissões
```cmd
# Executar como administrador
Run as Administrator: cmd.exe

# Verificar privilégios do serviço  
sc showsid EdgeVideoService
```