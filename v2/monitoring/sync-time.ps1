# Script para sincronizar relógio do WSL2/Docker com Windows
# Execute como Administrador

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Sincronizando relógio WSL2/Docker" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 1. Mostrar diferença atual
Write-Host "[1/5] Verificando diferença de tempo..." -ForegroundColor Yellow
$windowsTime = Get-Date
Write-Host "  Windows: $($windowsTime.ToString('yyyy-MM-dd HH:mm:ss'))" -ForegroundColor Gray

try {
    $prometheusTime = docker exec edge-video-prometheus date '+%Y-%m-%d %H:%M:%S' 2>$null
    Write-Host "  Prometheus: $prometheusTime" -ForegroundColor Gray

    if ($prometheusTime) {
        $promTime = [DateTime]::ParseExact($prometheusTime, 'yyyy-MM-dd HH:mm:ss', $null)
        $diff = ($windowsTime - $promTime).TotalSeconds
        Write-Host "  Diferença: $([Math]::Abs($diff)) segundos" -ForegroundColor $(if ([Math]::Abs($diff) -gt 10) { 'Red' } else { 'Green' })
    }
} catch {
    Write-Host "  Container não está rodando" -ForegroundColor Gray
}

Write-Host ""

# 2. Parar containers
Write-Host "[2/5] Parando containers..." -ForegroundColor Yellow
docker-compose -f "$PSScriptRoot\docker-compose.yml" down 2>&1 | Out-Null
Write-Host "  Containers parados" -ForegroundColor Green
Write-Host ""

# 3. Sincronizar WSL2
Write-Host "[3/5] Sincronizando WSL2..." -ForegroundColor Yellow
wsl --shutdown
Start-Sleep -Seconds 2

# Tentar reiniciar o serviço LxssManager
try {
    Restart-Service LxssManager -Force -ErrorAction Stop
    Write-Host "  WSL2 sincronizado" -ForegroundColor Green
} catch {
    Write-Host "  WSL2 reiniciado (LxssManager requer admin)" -ForegroundColor Yellow
}
Write-Host ""

# 4. Aguardar Docker Desktop
Write-Host "[4/5] Aguardando Docker Desktop..." -ForegroundColor Yellow
$timeout = 30
$elapsed = 0
while ($elapsed -lt $timeout) {
    try {
        docker ps 2>&1 | Out-Null
        if ($LASTEXITCODE -eq 0) {
            Write-Host "  Docker pronto!" -ForegroundColor Green
            break
        }
    } catch {}

    Start-Sleep -Seconds 1
    $elapsed++
    Write-Host "  Aguardando... ($elapsed/$timeout)" -ForegroundColor Gray
}
Write-Host ""

# 5. Reiniciar containers
Write-Host "[5/5] Reiniciando containers..." -ForegroundColor Yellow
docker-compose -f "$PSScriptRoot\docker-compose.yml" up -d
Write-Host ""

# Verificar nova diferença
Write-Host "Verificando sincronização..." -ForegroundColor Yellow
Start-Sleep -Seconds 3

$windowsTime = Get-Date
Write-Host "  Windows: $($windowsTime.ToString('yyyy-MM-dd HH:mm:ss'))" -ForegroundColor Gray

try {
    $prometheusTime = docker exec edge-video-prometheus date '+%Y-%m-%d %H:%M:%S' 2>$null
    Write-Host "  Prometheus: $prometheusTime" -ForegroundColor Gray

    if ($prometheusTime) {
        $promTime = [DateTime]::ParseExact($prometheusTime, 'yyyy-MM-dd HH:mm:ss', $null)
        $diff = ($windowsTime - $promTime).TotalSeconds
        $diffAbs = [Math]::Abs($diff)

        Write-Host ""
        if ($diffAbs -lt 2) {
            Write-Host "✅ SUCESSO! Diferença: $diffAbs segundos" -ForegroundColor Green
        } elseif ($diffAbs -lt 10) {
            Write-Host "⚠️ OK! Diferença: $diffAbs segundos (aceitável)" -ForegroundColor Yellow
        } else {
            Write-Host "❌ PROBLEMA! Diferença: $diffAbs segundos" -ForegroundColor Red
            Write-Host ""
            Write-Host "Tente reiniciar o Docker Desktop manualmente:" -ForegroundColor Yellow
            Write-Host "  1. Clique com botão direito no ícone do Docker" -ForegroundColor Gray
            Write-Host "  2. Selecione 'Restart Docker Desktop'" -ForegroundColor Gray
        }
    }
} catch {
    Write-Host "  Aguardando container inicializar..." -ForegroundColor Gray
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Acesse: http://localhost:3000" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
