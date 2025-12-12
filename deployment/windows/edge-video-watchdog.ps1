# ============================================================================
# Edge Video - Watchdog Service
# Monitora e reinicia o edge-video.exe se ele cair
# ============================================================================

$EdgeVideoPath = "C:\Users\maisfluxo\Desktop\edge-video-1.2"
$ExecutableName = "edge-video.exe"
$LogPath = "$EdgeVideoPath\logs"
$LogFile = "$LogPath\watchdog.log"

# Cria diretório de logs se não existir
if (!(Test-Path $LogPath)) {
    New-Item -ItemType Directory -Path $LogPath -Force | Out-Null
}

function Write-Log {
    param($Message)
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logMessage = "[$timestamp] $Message"
    Write-Host $logMessage
    Add-Content -Path $LogFile -Value $logMessage
}

function Start-EdgeVideo {
    Write-Log "Iniciando Edge Video..."

    Set-Location $EdgeVideoPath

    $process = Start-Process -FilePath "$EdgeVideoPath\$ExecutableName" `
                             -WorkingDirectory $EdgeVideoPath `
                             -PassThru `
                             -WindowStyle Hidden

    Write-Log "Edge Video iniciado (PID: $($process.Id))"
    return $process
}

function Stop-EdgeVideo {
    Write-Log "Parando Edge Video..."
    Get-Process -Name "edge-video" -ErrorAction SilentlyContinue | Stop-Process -Force
    Start-Sleep -Seconds 2
}

# ============================================================================
# LOOP PRINCIPAL DE MONITORAMENTO
# ============================================================================

Write-Log "=========================================="
Write-Log "Edge Video Watchdog iniciado"
Write-Log "=========================================="

$process = $null
$restartCount = 0

while ($true) {
    try {
        # Verifica se o processo está rodando
        $running = Get-Process -Name "edge-video" -ErrorAction SilentlyContinue

        if (!$running) {
            $restartCount++
            Write-Log "Edge Video não está rodando! (Restart #$restartCount)"

            # Aguarda 5 segundos antes de reiniciar
            Start-Sleep -Seconds 5

            # Garante que não há processos zumbis
            Stop-EdgeVideo

            # Inicia novamente
            $process = Start-EdgeVideo

            Write-Log "Edge Video reiniciado com sucesso"
        }

        # Aguarda 10 segundos antes da próxima verificação
        Start-Sleep -Seconds 10

    } catch {
        Write-Log "ERRO: $($_.Exception.Message)"
        Start-Sleep -Seconds 10
    }
}
