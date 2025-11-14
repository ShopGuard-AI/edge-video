# CI/CD Windows Installer

Este diretório contém a configuração completa para gerar instaladores Windows do Edge Video via GitHub Actions.

## Estrutura

```
.github/workflows/
├── windows-installer.yml    # GitHub Actions workflow
installer/windows/
├── edge-video-installer.nsi # Script NSIS para criar instalador
├── edge-video.ico          # Ícone do aplicativo (placeholder)
cmd/edge-video-service/
├── main.go                 # Service wrapper principal
├── service.go              # Funções de controle do serviço (Windows)
├── service_stub.go         # Stubs para outras plataformas
docs/windows/
├── README.md              # Documentação de instalação e uso
build-windows.sh           # Script local para teste
```

## Funcionalidades

### GitHub Actions Workflow
- **Trigger**: Push em tags `v*`, branches `main`/`feature/performance-improvements`, PRs, ou manual
- **Ambiente**: `windows-latest` com Go 1.24 e NSIS
- **Testes**: Roda `go test ./...` antes da build
- **Artefatos**: 
  - `EdgeVideoSetup-X.X.X.exe` (instalador NSIS)
  - `edge-video-service.exe` (binário do serviço)
  - `edge-video.exe` (binário CLI)
  - Checksums SHA256
- **Release**: Auto-release em tags Git com artefatos

### Windows Service
- **Instalação**: `edge-video-service.exe install`
- **Controle**: Start/stop via Services.msc ou linha de comando
- **Logs**: Windows Event Log (Application → EdgeVideoService)
- **Auto-start**: Configurado para iniciar com Windows
- **Recovery**: Restart automático em falhas

### Instalador NSIS
- **Interface gráfica** com páginas de licença, componentes, diretório
- **Instalação completa**: Binários + configuração + documentação + shortcuts
- **Registro automático**: Instala e inicia o serviço Windows
- **Desinstalação**: Remove tudo, preserva config opcionalmente

## Uso do CI/CD

### 1. Release Automático (Recomendado)
```bash
git tag v1.2.0
git push origin v1.2.0
```
- Executa workflow completo
- Cria GitHub Release com instalador
- Download: `EdgeVideoSetup-1.2.0.exe`

### 2. Build Manual
- GitHub → Actions → "Windows Installer Build" → "Run workflow"
- Gera artefatos sem release

### 3. Build Local (Desenvolvimento)
```bash
./build-windows.sh
# Gera binários em dist/ para teste
```

## Configuração do Workflow

### Variáveis de Ambiente
```yaml
GO_VERSION: '1.24'           # Versão do Go
PRODUCT_NAME: 'EdgeVideo'    # Nome do produto
PRODUCT_VERSION: '1.2.0'     # Versão (pode ser sobrescrita por tag)
```

### Dependências Instaladas
- **Go 1.24**: Compilação cross-platform 
- **NSIS**: Geração do instalador
- **Chocolatey**: Gerenciador de pacotes Windows

### Passos do Build
1. **Checkout** código
2. **Setup Go** environment  
3. **Install NSIS** via chocolatey
4. **Download** dependencies (`go mod download`)
5. **Test** (`go test ./...`)
6. **Build** binários Windows (service + CLI)
7. **Copy** configs e documentação
8. **Generate** instalador NSIS
9. **Calculate** checksums SHA256
10. **Upload** artefatos
11. **Release** (só em tags)

## Estrutura do Instalador

### Componentes
1. **Edge Video Service (Obrigatório)**
   - `edge-video-service.exe` - Binário principal
   - `edge-video.exe` - CLI para debug
   - `config/config.toml` - Configuração padrão
   - Registro no Windows Registry

2. **Start Menu Shortcuts (Opcional)**
   - Edge Video Service Manager
   - Configuration Editor  
   - Uninstaller
   - Desktop shortcut

3. **Install and Start Service (Opcional)**
   - Registra como Windows Service
   - Configura auto-start
   - Inicia imediatamente

### Diretórios de Instalação
```
C:\Program Files\T3Labs\EdgeVideo\
├── edge-video-service.exe
├── edge-video.exe
├── config\
│   └── config.toml
├── logs\                   # Logs locais (se habilitado)
├── docs\                   # Documentação
└── uninstall.exe
```

## Configuração Pós-Instalação

### 1. Configurar Câmeras
Editar `config\config.toml`:
```toml
[[cameras]]
id = "camera1"
url = "rtsp://admin:password@192.168.1.100:554/stream"
```

### 2. Configurar RabbitMQ
```toml
[amqp]
amqp_url = "amqp://user:pass@rabbitmq-server:5672/vhost"
exchange = "cameras"
```

### 3. Reiniciar Serviço
```cmd
net stop EdgeVideoService
net start EdgeVideoService
```

## Monitoramento

### Event Viewer
1. Abrir `eventvwr.msc`
2. Windows Logs → Application
3. Filtrar por Source: "EdgeVideoService"

### Linha de Comando
```cmd
# Status do serviço
sc query EdgeVideoService

# Logs PowerShell
Get-WinEvent -LogName Application | Where-Object {$_.ProviderName -eq "EdgeVideoService"}

# Teste console
edge-video-service.exe console
```

## Troubleshooting

### Build Falha
- **Go version**: Verificar compatibilidade
- **NSIS missing**: Chocolatey install pode falhar
- **Permissions**: Windows runner precisa admin para NSIS

### Service Não Inicia
- **Config inválida**: Verificar sintaxe TOML
- **Network**: Portas 554 (RTSP), 5672 (AMQP), 6379 (Redis)
- **Firewall**: Windows Defender pode bloquear

### Instalador Corrompe
- **Antivirus**: Pode quarentar .exe não assinado
- **Download**: Verificar SHA256 checksum
- **Admin**: Instalação requer privilégios elevados

## Melhorias Futuras

### Code Signing
```yaml
# Adicionar ao workflow
- name: Sign binaries
  run: |
    signtool sign /f cert.p12 /p ${{ secrets.CERT_PASSWORD }} dist/*.exe
```

### Auto-update
- Endpoint para check de versões
- Download e instalação automática
- Rollback em caso de falha

### Telemetria
- Metrics de instalação
- Health checks automáticos  
- Crash reporting

### Multi-platform
- Linux .deb packages
- macOS .pkg installers
- Docker registry pushes

## Licença

Este sistema de build mantém a licença do projeto principal. O instalador NSIS é open source (zlib license).