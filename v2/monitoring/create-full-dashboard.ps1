# Script para criar dashboard COMPLETO e PROFISSIONAL
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Criando Dashboard COMPLETO" -ForegroundColor Cyan
Write-Host "  Edge Video V2 - Computer Vision" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

$cred = [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes("admin:admin"))
$headers = @{
    Authorization = "Basic $cred"
    "Content-Type" = "application/json"
}

# Pegar datasource
Write-Host "[1/2] Obtendo datasource Prometheus..." -ForegroundColor Yellow
$datasources = Invoke-RestMethod -Uri "http://localhost:3000/api/datasources" -Headers $headers -Method Get
$promDs = $datasources | Where-Object { $_.type -eq "prometheus" } | Select-Object -First 1
Write-Host "  ✅ Datasource UID: $($promDs.uid)" -ForegroundColor Green
Write-Host ""

Write-Host "[2/2] Criando dashboard profissional..." -ForegroundColor Yellow

$dashboard = @{
    dashboard = @{
        title = "Edge Video V2 - Computer Vision Monitoring"
        uid = "edge-video-v2"
        timezone = "utc"
        schemaVersion = 38
        version = 1
        refresh = "5s"
        time = @{
            from = "now-15m"
            to = "now"
        }
        tags = @("edge-video", "computer-vision", "prometheus")
        panels = @(
            # ==================================================
            # ROW 1: SYSTEM OVERVIEW
            # ==================================================
            @{
                id = 100
                type = "row"
                title = "System Overview"
                gridPos = @{ x = 0; y = 0; w = 24; h = 1 }
                collapsed = $false
            }

            # Total Frames Published
            @{
                id = 1
                title = "Total Frames Published"
                type = "stat"
                gridPos = @{ x = 0; y = 1; w = 4; h = 4 }
                targets = @(
                    @{
                        expr = "sum(edge_video_frames_published_total)"
                        refId = "A"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
                options = @{
                    graphMode = "none"
                    textMode = "value_and_name"
                    colorMode = "value"
                    reduceOptions = @{
                        values = $false
                        calcs = @("lastNotNull")
                    }
                }
                fieldConfig = @{
                    defaults = @{
                        unit = "short"
                        color = @{ mode = "thresholds" }
                        thresholds = @{
                            mode = "absolute"
                            steps = @(
                                @{ value = $null; color = "green" }
                                @{ value = 10000; color = "yellow" }
                                @{ value = 50000; color = "red" }
                            )
                        }
                    }
                }
            }

            # Publisher ACK Rate
            @{
                id = 2
                title = "Publisher ACK Rate"
                type = "gauge"
                gridPos = @{ x = 4; y = 1; w = 4; h = 4 }
                targets = @(
                    @{
                        expr = "100 * edge_video_publisher_confirms_ack_total / (edge_video_publisher_confirms_ack_total + edge_video_publisher_confirms_nack_total + 0.0001)"
                        refId = "A"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
                fieldConfig = @{
                    defaults = @{
                        unit = "percent"
                        min = 0
                        max = 100
                        color = @{ mode = "thresholds" }
                        thresholds = @{
                            mode = "absolute"
                            steps = @(
                                @{ value = $null; color = "red" }
                                @{ value = 95; color = "yellow" }
                                @{ value = 99; color = "green" }
                            )
                        }
                    }
                }
            }

            # RAM Usage
            @{
                id = 3
                title = "RAM Usage"
                type = "stat"
                gridPos = @{ x = 8; y = 1; w = 4; h = 4 }
                targets = @(
                    @{
                        expr = "edge_video_system_ram_mb"
                        refId = "A"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
                options = @{
                    graphMode = "area"
                    textMode = "value_and_name"
                    colorMode = "value"
                }
                fieldConfig = @{
                    defaults = @{
                        unit = "decmbytes"
                        color = @{ mode = "thresholds" }
                        thresholds = @{
                            mode = "absolute"
                            steps = @(
                                @{ value = $null; color = "green" }
                                @{ value = 500; color = "yellow" }
                                @{ value = 1000; color = "red" }
                            )
                        }
                    }
                }
            }

            # Active Goroutines
            @{
                id = 4
                title = "Active Goroutines"
                type = "stat"
                gridPos = @{ x = 12; y = 1; w = 4; h = 4 }
                targets = @(
                    @{
                        expr = "edge_video_system_goroutines"
                        refId = "A"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
                options = @{
                    graphMode = "area"
                    textMode = "value_and_name"
                }
                fieldConfig = @{
                    defaults = @{
                        color = @{ mode = "thresholds" }
                        thresholds = @{
                            mode = "absolute"
                            steps = @(
                                @{ value = $null; color = "green" }
                                @{ value = 100; color = "yellow" }
                                @{ value = 200; color = "red" }
                            )
                        }
                    }
                }
            }

            # Uptime
            @{
                id = 5
                title = "Uptime"
                type = "stat"
                gridPos = @{ x = 16; y = 1; w = 4; h = 4 }
                targets = @(
                    @{
                        expr = "edge_video_uptime_seconds"
                        refId = "A"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
                fieldConfig = @{
                    defaults = @{
                        unit = "s"
                        color = @{ mode = "thresholds" }
                    }
                }
            }

            # Circuit Breakers OPEN
            @{
                id = 6
                title = "Circuit Breakers OPEN"
                type = "stat"
                gridPos = @{ x = 20; y = 1; w = 4; h = 4 }
                targets = @(
                    @{
                        expr = "count(edge_video_circuit_breaker_state == 1) OR vector(0)"
                        refId = "A"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
                fieldConfig = @{
                    defaults = @{
                        color = @{ mode = "thresholds" }
                        thresholds = @{
                            mode = "absolute"
                            steps = @(
                                @{ value = $null; color = "green" }
                                @{ value = 1; color = "red" }
                            )
                        }
                    }
                }
            }

            # ==================================================
            # ROW 2: CAMERA PERFORMANCE
            # ==================================================
            @{
                id = 200
                type = "row"
                title = "Camera Performance"
                gridPos = @{ x = 0; y = 5; w = 24; h = 1 }
                collapsed = $false
            }

            # Frames Published by Camera
            @{
                id = 7
                title = "Frames Published by Camera"
                type = "timeseries"
                gridPos = @{ x = 0; y = 6; w = 8; h = 8 }
                targets = @(
                    @{
                        expr = "edge_video_frames_published_total"
                        refId = "A"
                        legendFormat = "{{camera_id}}"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
                fieldConfig = @{
                    defaults = @{
                        custom = @{
                            drawStyle = "line"
                            lineInterpolation = "smooth"
                            showPoints = "never"
                            fillOpacity = 10
                        }
                    }
                }
            }

            # Publish Latency per Camera
            @{
                id = 8
                title = "Publish Latency per Camera (ms)"
                type = "timeseries"
                gridPos = @{ x = 8; y = 6; w = 8; h = 8 }
                targets = @(
                    @{
                        expr = "edge_video_publish_latency_ms"
                        refId = "A"
                        legendFormat = "{{camera_id}}"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
                fieldConfig = @{
                    defaults = @{
                        unit = "ms"
                        custom = @{
                            drawStyle = "line"
                            lineInterpolation = "smooth"
                        }
                    }
                }
            }

            # Frame Flow per Camera
            @{
                id = 9
                title = "Frame Flow per Camera"
                type = "timeseries"
                gridPos = @{ x = 16; y = 6; w = 8; h = 8 }
                targets = @(
                    @{
                        expr = "rate(edge_video_frames_received_total[1m]) * 60"
                        refId = "A"
                        legendFormat = "{{camera_id}} - Received"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                    @{
                        expr = "rate(edge_video_frames_published_total[1m]) * 60"
                        refId = "B"
                        legendFormat = "{{camera_id}} - Published"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                    @{
                        expr = "rate(edge_video_frames_dropped_total[1m]) * 60"
                        refId = "C"
                        legendFormat = "{{camera_id}} - Dropped"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
                fieldConfig = @{
                    defaults = @{
                        unit = "fps"
                    }
                }
            }

            # ==================================================
            # ROW 3: SYSTEM RESOURCES & HEALTH
            # ==================================================
            @{
                id = 300
                type = "row"
                title = "System Resources and Health"
                gridPos = @{ x = 0; y = 14; w = 24; h = 1 }
                collapsed = $false
            }

            # Memory Usage Over Time
            @{
                id = 10
                title = "Memory Usage Over Time"
                type = "timeseries"
                gridPos = @{ x = 0; y = 15; w = 8; h = 8 }
                targets = @(
                    @{
                        expr = "edge_video_system_ram_mb"
                        refId = "A"
                        legendFormat = "RAM Usage"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
                fieldConfig = @{
                    defaults = @{
                        unit = "decmbytes"
                        custom = @{
                            fillOpacity = 20
                        }
                    }
                }
            }

            # Goroutines Over Time
            @{
                id = 11
                title = "Goroutines Over Time"
                type = "timeseries"
                gridPos = @{ x = 8; y = 15; w = 8; h = 8 }
                targets = @(
                    @{
                        expr = "edge_video_system_goroutines"
                        refId = "A"
                        legendFormat = "Active Goroutines"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
            }

            # Publisher Confirms (ACK/NACK Rate)
            @{
                id = 12
                title = "Publisher Confirms (ACK/NACK Rate)"
                type = "timeseries"
                gridPos = @{ x = 16; y = 15; w = 8; h = 8 }
                targets = @(
                    @{
                        expr = "rate(edge_video_publisher_confirms_ack_total[1m]) * 60"
                        refId = "A"
                        legendFormat = "ACK/min"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                    @{
                        expr = "rate(edge_video_publisher_confirms_nack_total[1m]) * 60"
                        refId = "B"
                        legendFormat = "NACK/min"
                        datasource = @{ type = "prometheus"; uid = $promDs.uid }
                    }
                )
                fieldConfig = @{
                    defaults = @{
                        custom = @{
                            fillOpacity = 20
                        }
                    }
                }
            }
        )
    }
    overwrite = $true
    message = "Dashboard completo e profissional"
} | ConvertTo-Json -Depth 25

# Importar dashboard
try {
    $result = Invoke-RestMethod -Uri "http://localhost:3000/api/dashboards/db" -Headers $headers -Method Post -Body $dashboard
    Write-Host "  ✅ Dashboard criado com sucesso!" -ForegroundColor Green
    Write-Host ""
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "  DASHBOARD PRONTO!" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Acesse: http://localhost:3000$($result.url)" -ForegroundColor Green
    Write-Host ""
    Write-Host "Paineis incluidos:" -ForegroundColor White
    Write-Host "  - 6 paineis de System Overview" -ForegroundColor Gray
    Write-Host "  - 3 paineis de Camera Performance" -ForegroundColor Gray
    Write-Host "  - 3 paineis de System Resources and Health" -ForegroundColor Gray
    Write-Host "  - Total: 12 paineis profissionais" -ForegroundColor Gray
    Write-Host ""
} catch {
    Write-Host "  ❌ Erro: $_" -ForegroundColor Red
}
