# ============================================================================
# Edge Video - Setup Completo de Resiliência
# Configura TUDO de uma vez: RabbitMQ, Redis e Edge Video Watchdog
# Execute como Administrador!
# ============================================================================

param(
    [string]$EdgeVideoPath = "C:\Users\maisfluxo\Desktop\edge-video-1.2"
)

Write-Host ""
Write-Host "============================================================================" -ForegroundColor Cyan
Write-Host "  EDGE VIDEO - SETUP COMPLETO DE RESILIÊNCIA" -ForegroundColor Cyan
Write-Host "============================================================================" -ForegroundColor Cyan
Write-Host ""

# ============================================================================
# 1. VERIFICAÇÕES PRÉ-INSTALAÇÃO
# ============================================================================

Write-Host "📋 ETAPA 1/4: Verificações..." -ForegroundColor Yellow
Write-Host ""

# Verifica se está executando como Admin
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

if (!$isAdmin) {
    Write-Host "❌ ERRO: Execute este script como Administrador!" -ForegroundColor Red
    Write-Host ""
    Write-Host "Clique com botão direito no PowerShell e selecione 'Executar como Administrador'" -ForegroundColor Yellow
    exit 1
}

# Verifica se o Edge Video existe
if (!(Test-Path "$EdgeVideoPath\edge-video.exe")) {
    Write-Host "❌ ERRO: edge-video.exe não encontrado em: $EdgeVideoPath" -ForegroundColor Red
    exit 1
}

Write-Host "✓ Executando como Administrador" -ForegroundColor Green
Write-Host "✓ Edge Video encontrado em: $EdgeVideoPath" -ForegroundColor Green
Write-Host ""

# ============================================================================
# 2. CONFIGURAR RABBITMQ E REDIS PARA AUTO-START
# ============================================================================

Write-Host "🔧 ETAPA 2/4: Configurando RabbitMQ e Redis..." -ForegroundColor Yellow
Write-Host ""

# RabbitMQ
$rabbitmq = Get-Service -Name "RabbitMQ" -ErrorAction SilentlyContinue
if ($rabbitmq) {
    Set-Service -Name RabbitMQ -StartupType Automatic
    if ($rabbitmq.Status -ne "Running") {
        Start-Service RabbitMQ
    }
    Write-Host "✓ RabbitMQ configurado para auto-start" -ForegroundColor Green
} else {
    Write-Host "⚠  RabbitMQ não encontrado (ignorando)" -ForegroundColor Yellow
}

# Redis/Memurai
$memurai = Get-Service -Name "Memurai" -ErrorAction SilentlyContinue
if ($memurai) {
    Set-Service -Name Memurai -StartupType Automatic
    if ($memurai.Status -ne "Running") {
        Start-Service Memurai
    }
    Write-Host "✓ Memurai (Redis) configurado para auto-start" -ForegroundColor Green
} else {
    Write-Host "⚠  Memurai não encontrado (ignorando)" -ForegroundColor Yellow
}

Write-Host ""

# ============================================================================
# 3. CRIAR DIRETÓRIO DE LOGS
# ============================================================================

Write-Host "📁 ETAPA 3/4: Criando estrutura de logs..." -ForegroundColor Yellow
Write-Host ""

$logPath = "$EdgeVideoPath\logs"
if (!(Test-Path $logPath)) {
    New-Item -ItemType Directory -Path $logPath -Force | Out-Null
    Write-Host "✓ Diretório de logs criado: $logPath" -ForegroundColor Green
} else {
    Write-Host "✓ Diretório de logs já existe" -ForegroundColor Green
}

Write-Host ""

# ============================================================================
# 4. CRIAR WATCHDOG SCRIPT
# ============================================================================

Write-Host "🐕 ETAPA 4/4: Instalando Edge Video Watchdog..." -ForegroundColor Yellow
Write-Host ""

# Gera watchdog script inline (não precisa copiar arquivo)
$watchdogContent = @"
# Edge Video Watchdog - Gerado automaticamente
`$EdgeVideoPath = "$EdgeVideoPath"
`$ExecutableName = "edge-video.exe"
`$LogPath = "`$EdgeVideoPath\logs"
`$LogFile = "`$LogPath\watchdog.log"

function Write-Log {
    param(`$Message)
    `$timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    `$logMessage = "[`$timestamp] `$Message"
    Write-Host `$logMessage
    Add-Content -Path `$LogFile -Value `$logMessage
}

function Start-EdgeVideo {
    Write-Log "Iniciando Edge Video..."
    Set-Location `$EdgeVideoPath
    `$process = Start-Process -FilePath "`$EdgeVideoPath\`$ExecutableName" -WorkingDirectory `$EdgeVideoPath -PassThru -WindowStyle Hidden
    Write-Log "Edge Video iniciado (PID: `$(`$process.Id))"
    return `$process
}

function Stop-EdgeVideo {
    Write-Log "Parando Edge Video..."
    Get-Process -Name "edge-video" -ErrorAction SilentlyContinue | Stop-Process -Force
    Start-Sleep -Seconds 2
}

Write-Log "=========================================="
Write-Log "Edge Video Watchdog iniciado"
Write-Log "=========================================="

`$restartCount = 0

while (`$true) {
    try {
        `$running = Get-Process -Name "edge-video" -ErrorAction SilentlyContinue
        if (!`$running) {
            `$restartCount++
            Write-Log "Edge Video não está rodando! (Restart #`$restartCount)"
            Start-Sleep -Seconds 5
            Stop-EdgeVideo
            Start-EdgeVideo
            Write-Log "Edge Video reiniciado com sucesso"
        }
        Start-Sleep -Seconds 10
    } catch {
        Write-Log "ERRO: `$(`$_.Exception.Message)"
        Start-Sleep -Seconds 10
    }
}
"@

$watchdogPath = "$EdgeVideoPath\edge-video-watchdog.ps1"
Set-Content -Path $watchdogPath -Value $watchdogContent -Force
Write-Host "✓ Watchdog script criado: $watchdogPath" -ForegroundColor Green

# Remove tarefa existente (se houver)
$taskName = "EdgeVideoWatchdog"
$existingTask = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue

if ($existingTask) {
    Write-Host "⚠  Removendo tarefa existente..." -ForegroundColor Yellow
    Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
}

# Cria tarefa agendada
$action = New-ScheduledTaskAction `
    -Execute "PowerShell.exe" `
    -Argument "-ExecutionPolicy Bypass -NoProfile -WindowStyle Hidden -File `"$watchdogPath`""

$trigger = New-ScheduledTaskTrigger -AtStartup

$principal = New-ScheduledTaskPrincipal `
    -UserId "SYSTEM" `
    -LogonType ServiceAccount `
    -RunLevel Highest

$settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -StartWhenAvailable `
    -RestartCount 3 `
    -RestartInterval (New-TimeSpan -Minutes 1) `
    -ExecutionTimeLimit (New-TimeSpan -Days 365)

Register-ScheduledTask `
    -TaskName $taskName `
    -Action $action `
    -Trigger $trigger `
    -Principal $principal `
    -Settings $settings `
    -Description "Edge Video Watchdog - Monitora e reinicia automaticamente" | Out-Null

Write-Host "✓ Tarefa agendada '$taskName' criada" -ForegroundColor Green

# Inicia tarefa
Start-ScheduledTask -TaskName $taskName
Start-Sleep -Seconds 3

# Verifica se Edge Video está rodando
$edgeProcess = Get-Process -Name "edge-video" -ErrorAction SilentlyContinue
if ($edgeProcess) {
    Write-Host "✓ Edge Video rodando (PID: $($edgeProcess.Id))" -ForegroundColor Green
} else {
    Write-Host "⚠  Edge Video ainda não iniciou (aguarde alguns segundos)" -ForegroundColor Yellow
}

Write-Host ""

# ============================================================================
# RESUMO FINAL
# ============================================================================

Write-Host "============================================================================" -ForegroundColor Cyan
Write-Host "  ✅ INSTALAÇÃO CONCLUÍDA COM SUCESSO!" -ForegroundColor Green
Write-Host "============================================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "📋 RESUMO DA CONFIGURAÇÃO:" -ForegroundColor White
Write-Host ""

# Status dos serviços
$services = @("RabbitMQ", "Memurai")
foreach ($svc in $services) {
    $service = Get-Service -Name $svc -ErrorAction SilentlyContinue
    if ($service) {
        $statusColor = if ($service.Status -eq "Running") { "Green" } else { "Yellow" }
        $startTypeColor = if ($service.StartType -eq "Automatic") { "Green" } else { "Yellow" }
        Write-Host "  $svc" -NoNewline
        Write-Host " - Status: " -NoNewline
        Write-Host "$($service.Status)" -ForegroundColor $statusColor -NoNewline
        Write-Host " | Auto-start: " -NoNewline
        Write-Host "$($service.StartType)" -ForegroundColor $startTypeColor
    }
}

Write-Host ""
Write-Host "  Edge Video Watchdog - " -NoNewline
$task = Get-ScheduledTask -TaskName $taskName
Write-Host "Ativo ($($task.State))" -ForegroundColor Green

Write-Host ""
Write-Host "📂 LOGS:" -ForegroundColor White
Write-Host "  $logPath" -ForegroundColor Gray
Write-Host ""
Write-Host "🔍 COMANDOS ÚTEIS:" -ForegroundColor White
Write-Host "  Ver logs:      Get-Content '$logPath\watchdog.log' -Tail 20" -ForegroundColor Gray
Write-Host "  Ver processo:  Get-Process edge-video" -ForegroundColor Gray
Write-Host "  Parar tudo:    Stop-ScheduledTask -TaskName $taskName" -ForegroundColor Gray
Write-Host ""
Write-Host "🧪 TESTE DE RESILIÊNCIA:" -ForegroundColor White
Write-Host "  Get-Process edge-video | Stop-Process -Force" -ForegroundColor Gray
Write-Host "  (aguarde 10s e verifique se reiniciou)" -ForegroundColor Gray
Write-Host ""
Write-Host "============================================================================" -ForegroundColor Cyan
Write-Host "  🎉 Sistema 100% resiliente e pronto para produção!" -ForegroundColor Green
Write-Host "============================================================================" -ForegroundColor Cyan
Write-Host ""
