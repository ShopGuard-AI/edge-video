# Script de instalação do Docker Desktop via winget
# Execute como Administrador

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Instalando Docker Desktop" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Verifica se está rodando como administrador
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

if (-not $isAdmin) {
    Write-Host "ERRO: Execute este script como Administrador!" -ForegroundColor Red
    Write-Host ""
    Write-Host "Clique com botao direito no PowerShell e selecione 'Executar como Administrador'" -ForegroundColor Yellow
    Write-Host ""
    pause
    exit 1
}

Write-Host "[1/3] Verificando winget..." -ForegroundColor Yellow

# Verifica se winget está disponível
try {
    $wingetVersion = winget --version
    Write-Host "  winget encontrado: $wingetVersion" -ForegroundColor Green
} catch {
    Write-Host "  ERRO: winget nao encontrado!" -ForegroundColor Red
    Write-Host ""
    Write-Host "Instale o App Installer da Microsoft Store ou baixe Docker manualmente:" -ForegroundColor Yellow
    Write-Host "  https://www.docker.com/products/docker-desktop/" -ForegroundColor Cyan
    Write-Host ""
    pause
    exit 1
}

Write-Host ""
Write-Host "[2/3] Instalando Docker Desktop..." -ForegroundColor Yellow
Write-Host "  (Isso pode demorar alguns minutos)" -ForegroundColor Gray
Write-Host ""

# Instala Docker Desktop
winget install -e --id Docker.DockerDesktop --accept-package-agreements --accept-source-agreements

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "[3/3] Instalacao concluida!" -ForegroundColor Green
    Write-Host ""
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "  PROXIMOS PASSOS" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "1. REINICIE o computador (necessario)" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "2. Apos reiniciar, abra Docker Desktop:" -ForegroundColor Yellow
    Write-Host "   - Procure 'Docker Desktop' no menu Iniciar" -ForegroundColor Gray
    Write-Host "   - Aguarde ele inicializar completamente (icone na bandeja)" -ForegroundColor Gray
    Write-Host ""
    Write-Host "3. Teste se Docker esta funcionando:" -ForegroundColor Yellow
    Write-Host "   docker --version" -ForegroundColor Cyan
    Write-Host "   docker-compose --version" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "4. Inicie Prometheus + Grafana:" -ForegroundColor Yellow
    Write-Host "   cd D:\Users\rafa2\OneDrive\Desktop\edge-video\v2\monitoring" -ForegroundColor Cyan
    Write-Host "   docker-compose up -d" -ForegroundColor Cyan
    Write-Host ""
} else {
    Write-Host ""
    Write-Host "ERRO na instalacao!" -ForegroundColor Red
    Write-Host ""
    Write-Host "Tente instalar manualmente:" -ForegroundColor Yellow
    Write-Host "  https://www.docker.com/products/docker-desktop/" -ForegroundColor Cyan
    Write-Host ""
}

pause
