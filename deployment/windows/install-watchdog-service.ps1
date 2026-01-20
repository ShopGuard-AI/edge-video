# ============================================================================
# Edge Video - Instalador do Watchdog Service
# Cria tarefa agendada do Windows para monitoramento automático
# Execute como Administrador!
# ============================================================================

param(
    [string]$EdgeVideoPath = "C:\Users\maisfluxo\Desktop\edge-video-1.2",
    [string]$WatchdogScript = "edge-video-watchdog.ps1"
)

Write-Host "=========================================="
Write-Host "Edge Video - Instalador Watchdog Service"
Write-Host "=========================================="
Write-Host ""

# Verifica se está executando como Admin
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

if (!$isAdmin) {
    Write-Host "❌ ERRO: Execute este script como Administrador!" -ForegroundColor Red
    Write-Host ""
    Write-Host "Clique com botão direito no PowerShell e selecione 'Executar como Administrador'" -ForegroundColor Yellow
    exit 1
}

# Cria diretório de logs
$logPath = "$EdgeVideoPath\logs"
if (!(Test-Path $logPath)) {
    New-Item -ItemType Directory -Path $logPath -Force | Out-Null
    Write-Host "✓ Diretório de logs criado: $logPath" -ForegroundColor Green
}

# Copia watchdog script para o diretório do edge-video
$sourceScript = $PSScriptRoot + "\$WatchdogScript"
$destScript = "$EdgeVideoPath\$WatchdogScript"

if (Test-Path $sourceScript) {
    Copy-Item -Path $sourceScript -Destination $destScript -Force
    Write-Host "✓ Watchdog script copiado para: $destScript" -ForegroundColor Green
} else {
    Write-Host "❌ ERRO: Script $WatchdogScript não encontrado em $PSScriptRoot" -ForegroundColor Red
    exit 1
}

# Remove tarefa existente (se houver)
$taskName = "EdgeVideoWatchdog"
$existingTask = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue

if ($existingTask) {
    Write-Host "⚠  Removendo tarefa existente..." -ForegroundColor Yellow
    Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
}

# Cria ação da tarefa (executar PowerShell com o watchdog script)
$action = New-ScheduledTaskAction `
    -Execute "PowerShell.exe" `
    -Argument "-ExecutionPolicy Bypass -NoProfile -WindowStyle Hidden -File `"$destScript`""

# Trigger: Iniciar no boot do sistema
$trigger = New-ScheduledTaskTrigger -AtStartup

# Configurações: Executar sempre, mesmo sem usuário logado
$principal = New-ScheduledTaskPrincipal `
    -UserId "SYSTEM" `
    -LogonType ServiceAccount `
    -RunLevel Highest

# Settings: Permitir execução sob demanda, reiniciar se falhar
$settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -StartWhenAvailable `
    -RestartCount 3 `
    -RestartInterval (New-TimeSpan -Minutes 1) `
    -ExecutionTimeLimit (New-TimeSpan -Days 365)

# Registra tarefa agendada
Register-ScheduledTask `
    -TaskName $taskName `
    -Action $action `
    -Trigger $trigger `
    -Principal $principal `
    -Settings $settings `
    -Description "Edge Video Watchdog - Monitora e reinicia o edge-video.exe automaticamente"

Write-Host "✓ Tarefa agendada '$taskName' criada com sucesso!" -ForegroundColor Green

# Inicia tarefa imediatamente
Write-Host ""
Write-Host "Iniciando watchdog agora..." -ForegroundColor Cyan
Start-ScheduledTask -TaskName $taskName
Start-Sleep -Seconds 2

# Verifica status
$task = Get-ScheduledTask -TaskName $taskName
Write-Host "✓ Watchdog iniciado! Status: $($task.State)" -ForegroundColor Green

Write-Host ""
Write-Host "=========================================="
Write-Host "✅ INSTALAÇÃO CONCLUÍDA COM SUCESSO!"
Write-Host "=========================================="
Write-Host ""
Write-Host "O Edge Video agora irá:" -ForegroundColor Cyan
Write-Host "  • Iniciar automaticamente no boot do Windows"
Write-Host "  • Reiniciar automaticamente se cair"
Write-Host "  • Gravar logs em: $logPath"
Write-Host ""
Write-Host "Comandos úteis:" -ForegroundColor Yellow
Write-Host "  Ver status:    Get-ScheduledTask -TaskName $taskName"
Write-Host "  Parar:         Stop-ScheduledTask -TaskName $taskName"
Write-Host "  Iniciar:       Start-ScheduledTask -TaskName $taskName"
Write-Host "  Desinstalar:   Unregister-ScheduledTask -TaskName $taskName -Confirm:`$false"
Write-Host ""
