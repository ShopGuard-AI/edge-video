# Script para criar dashboard funcional no Grafana
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Criando Dashboard Edge Video V2" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

$cred = [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes("admin:admin"))
$headers = @{
    Authorization = "Basic $cred"
    "Content-Type" = "application/json"
}

# Pegar UID do datasource Prometheus
Write-Host "[1/3] Obtendo datasource..." -ForegroundColor Yellow
$datasources = Invoke-RestMethod -Uri "http://localhost:3000/api/datasources" -Headers $headers -Method Get
$promDs = $datasources | Where-Object { $_.type -eq "prometheus" } | Select-Object -First 1

if (-not $promDs) {
    Write-Host "  ❌ Datasource Prometheus não encontrado!" -ForegroundColor Red
    exit 1
}

Write-Host "  ✅ Datasource: $($promDs.name) (UID: $($promDs.uid))" -ForegroundColor Green
Write-Host ""

# Criar dashboard
Write-Host "[2/3] Criando dashboard..." -ForegroundColor Yellow

$dashboard = @{
    dashboard = @{
        title = "Edge Video V2 - FIXED"
        uid = "edge-video-v2-fixed"
        timezone = "utc"
        schemaVersion = 38
        version = 1
        refresh = "5s"
        time = @{
            from = "now-1h"
            to = "now"
        }
        panels = @(
            # Total Frames Published
            @{
                id = 1
                title = "Total Frames Published"
                type = "stat"
                gridPos = @{ x = 0; y = 0; w = 6; h = 4 }
                targets = @(
                    @{
                        expr = "sum(edge_video_frames_published_total)"
                        refId = "A"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
                options = @{
                    reduceOptions = @{
                        values = $false
                        calcs = @("lastNotNull")
                    }
                }
            }
            # Publisher ACK Rate
            @{
                id = 2
                title = "Publisher ACK Rate"
                type = "stat"
                gridPos = @{ x = 6; y = 0; w = 6; h = 4 }
                targets = @(
                    @{
                        expr = "100 * edge_video_publisher_confirms_ack_total / (edge_video_publisher_confirms_ack_total + edge_video_publisher_confirms_nack_total)"
                        refId = "A"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
                fieldConfig = @{
                    defaults = @{
                        unit = "percent"
                        max = 100
                        min = 0
                    }
                }
            }
            # RAM Usage
            @{
                id = 3
                title = "RAM Usage (MB)"
                type = "stat"
                gridPos = @{ x = 12; y = 0; w = 6; h = 4 }
                targets = @(
                    @{
                        expr = "edge_video_system_ram_mb"
                        refId = "A"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
                fieldConfig = @{
                    defaults = @{
                        unit = "decmbytes"
                    }
                }
            }
            # Goroutines
            @{
                id = 4
                title = "Active Goroutines"
                type = "stat"
                gridPos = @{ x = 18; y = 0; w = 6; h = 4 }
                targets = @(
                    @{
                        expr = "edge_video_system_goroutines"
                        refId = "A"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
            }
            # Frames by Camera (Time Series)
            @{
                id = 5
                title = "Frames Published by Camera"
                type = "timeseries"
                gridPos = @{ x = 0; y = 4; w = 12; h = 8 }
                targets = @(
                    @{
                        expr = "edge_video_frames_published_total"
                        refId = "A"
                        legendFormat = "{{camera_id}}"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
            }
            # RAM Over Time
            @{
                id = 6
                title = "RAM Usage Over Time"
                type = "timeseries"
                gridPos = @{ x = 12; y = 4; w = 12; h = 8 }
                targets = @(
                    @{
                        expr = "edge_video_system_ram_mb"
                        refId = "A"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
                fieldConfig = @{
                    defaults = @{
                        unit = "decmbytes"
                    }
                }
            }
            # Publisher Confirms
            @{
                id = 7
                title = "Publisher Confirms (ACK/NACK)"
                type = "timeseries"
                gridPos = @{ x = 0; y = 12; w = 12; h = 8 }
                targets = @(
                    @{
                        expr = "rate(edge_video_publisher_confirms_ack_total[1m])"
                        refId = "A"
                        legendFormat = "ACK/s"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                    @{
                        expr = "rate(edge_video_publisher_confirms_nack_total[1m])"
                        refId = "B"
                        legendFormat = "NACK/s"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
            }
            # Frames Dropped
            @{
                id = 8
                title = "Frames Dropped by Camera"
                type = "timeseries"
                gridPos = @{ x = 12; y = 12; w = 12; h = 8 }
                targets = @(
                    @{
                        expr = "edge_video_frames_dropped_total"
                        refId = "A"
                        legendFormat = "{{camera_id}}"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
            }
        )
    }
    overwrite = $true
    message = "Dashboard criado via script"
} | ConvertTo-Json -Depth 20

Write-Host "  ✅ Dashboard JSON criado" -ForegroundColor Green
Write-Host ""

# Enviar para Grafana
Write-Host "[3/3] Importando dashboard..." -ForegroundColor Yellow
try {
    $result = Invoke-RestMethod -Uri "http://localhost:3000/api/dashboards/db" -Headers $headers -Method Post -Body $dashboard
    Write-Host "  ✅ Dashboard criado com sucesso!" -ForegroundColor Green
    Write-Host "  UID: $($result.uid)" -ForegroundColor Gray
    Write-Host "  URL: $($result.url)" -ForegroundColor Gray
} catch {
    Write-Host "  ❌ Erro ao criar dashboard: $_" -ForegroundColor Red
    Write-Host "  Detalhes: $($_.Exception.Message)" -ForegroundColor Gray
    exit 1
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  DASHBOARD CRIADO!" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Acesse: http://localhost:3000/d/edge-video-v2-fixed" -ForegroundColor Green
Write-Host ""
Write-Host "Login: admin / admin" -ForegroundColor Gray
Write-Host ""
