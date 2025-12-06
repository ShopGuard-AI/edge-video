# Script para diagnosticar e corrigir Grafana
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  DEBUG GRAFANA + PROMETHEUS" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 1. Testar Prometheus
Write-Host "[1/5] Testando Prometheus..." -ForegroundColor Yellow
try {
    $promTest = Invoke-RestMethod -Uri "http://localhost:9090/api/v1/query?query=up" -Method Get
    if ($promTest.status -eq "success") {
        Write-Host "  ✅ Prometheus OK" -ForegroundColor Green
        Write-Host "  Targets ativos: $($promTest.data.result.Count)" -ForegroundColor Gray
    }
} catch {
    Write-Host "  ❌ Prometheus não responde!" -ForegroundColor Red
    exit 1
}
Write-Host ""

# 2. Testar query de métricas do Edge Video
Write-Host "[2/5] Testando métricas Edge Video..." -ForegroundColor Yellow
try {
    $metricsTest = Invoke-RestMethod -Uri "http://localhost:9090/api/v1/query?query=edge_video_frames_published_total" -Method Get
    if ($metricsTest.data.result.Count -gt 0) {
        Write-Host "  ✅ Métricas encontradas: $($metricsTest.data.result.Count) séries" -ForegroundColor Green
        foreach ($serie in $metricsTest.data.result) {
            $cam = $serie.metric.camera_id
            $val = $serie.value[1]
            Write-Host "    - $cam : $val frames" -ForegroundColor Gray
        }
    } else {
        Write-Host "  ❌ Nenhuma métrica encontrada!" -ForegroundColor Red
        Write-Host "  Edge Video está rodando?" -ForegroundColor Yellow
    }
} catch {
    Write-Host "  ❌ Erro ao consultar métricas" -ForegroundColor Red
}
Write-Host ""

# 3. Testar Grafana
Write-Host "[3/5] Testando Grafana..." -ForegroundColor Yellow
$cred = [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes("admin:admin"))
$headers = @{
    Authorization = "Basic $cred"
}

try {
    $grafanaHealth = Invoke-RestMethod -Uri "http://localhost:3000/api/health" -Method Get
    Write-Host "  ✅ Grafana OK" -ForegroundColor Green
} catch {
    Write-Host "  ❌ Grafana não responde!" -ForegroundColor Red
    exit 1
}
Write-Host ""

# 4. Verificar datasources
Write-Host "[4/5] Verificando datasources..." -ForegroundColor Yellow
try {
    $datasources = Invoke-RestMethod -Uri "http://localhost:3000/api/datasources" -Headers $headers -Method Get

    if ($datasources.Count -eq 0) {
        Write-Host "  ⚠️  Nenhum datasource configurado!" -ForegroundColor Yellow
        Write-Host "  Criando datasource Prometheus..." -ForegroundColor Yellow

        $newDs = @{
            name = "Prometheus"
            type = "prometheus"
            url = "http://prometheus:9090"
            access = "proxy"
            isDefault = $true
            jsonData = @{
                timeInterval = "5s"
                httpMethod = "POST"
            }
        } | ConvertTo-Json

        Invoke-RestMethod -Uri "http://localhost:3000/api/datasources" -Headers $headers -Method Post -Body $newDs -ContentType "application/json"
        Write-Host "  ✅ Datasource criado!" -ForegroundColor Green
    } else {
        foreach ($ds in $datasources) {
            Write-Host "  Datasource: $($ds.name)" -ForegroundColor Gray
            Write-Host "    - Type: $($ds.type)" -ForegroundColor Gray
            Write-Host "    - URL: $($ds.url)" -ForegroundColor Gray
            Write-Host "    - UID: $($ds.uid)" -ForegroundColor Gray

            # Testar datasource
            Write-Host "    - Testando conexão..." -ForegroundColor Gray
            try {
                $testResult = Invoke-RestMethod -Uri "http://localhost:3000/api/datasources/$($ds.id)/health" -Headers $headers -Method Get
                if ($testResult.status -eq "OK") {
                    Write-Host "    - ✅ Conexão OK" -ForegroundColor Green
                } else {
                    Write-Host "    - ❌ Falha: $($testResult.message)" -ForegroundColor Red
                }
            } catch {
                Write-Host "    - ❌ Erro ao testar: $_" -ForegroundColor Red
            }
        }
    }
} catch {
    Write-Host "  ❌ Erro ao verificar datasources: $_" -ForegroundColor Red
}
Write-Host ""

# 5. Testar query via Grafana
Write-Host "[5/5] Testando query via Grafana..." -ForegroundColor Yellow
try {
    $datasources = Invoke-RestMethod -Uri "http://localhost:3000/api/datasources" -Headers $headers -Method Get
    if ($datasources.Count -gt 0) {
        $dsUid = $datasources[0].uid

        $queryBody = @{
            queries = @(
                @{
                    refId = "A"
                    expr = "edge_video_frames_published_total"
                    datasource = @{
                        type = "prometheus"
                        uid = $dsUid
                    }
                }
            )
            from = [string]([DateTimeOffset]::Now.AddHours(-1).ToUnixTimeMilliseconds())
            to = [string]([DateTimeOffset]::Now.ToUnixTimeMilliseconds())
        } | ConvertTo-Json -Depth 10

        $queryResult = Invoke-RestMethod -Uri "http://localhost:3000/api/ds/query" -Headers $headers -Method Post -Body $queryBody -ContentType "application/json"

        if ($queryResult.results.A.frames.Count -gt 0) {
            Write-Host "  ✅ Query retornou dados!" -ForegroundColor Green
            Write-Host "  Séries: $($queryResult.results.A.frames[0].data.values.Count)" -ForegroundColor Gray
        } else {
            Write-Host "  ❌ Query não retornou dados" -ForegroundColor Red
            Write-Host "  Response: $($queryResult | ConvertTo-Json -Depth 3)" -ForegroundColor Gray
        }
    }
} catch {
    Write-Host "  ❌ Erro ao testar query: $_" -ForegroundColor Red
    Write-Host "  Detalhes: $($_.Exception.Message)" -ForegroundColor Gray
}
Write-Host ""

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  RESUMO" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "1. Prometheus: http://localhost:9090" -ForegroundColor White
Write-Host "2. Grafana: http://localhost:3000" -ForegroundColor White
Write-Host "3. Métricas: http://localhost:2112/metrics" -ForegroundColor White
Write-Host ""
Write-Host "Se ainda não funcionar:" -ForegroundColor Yellow
Write-Host "  1. Verifique se Edge Video está rodando" -ForegroundColor Gray
Write-Host "  2. Aguarde 30 segundos para acumular dados" -ForegroundColor Gray
Write-Host "  3. No dashboard, mude range para 'Last 6 hours'" -ForegroundColor Gray
Write-Host ""
