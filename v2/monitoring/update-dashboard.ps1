# Update Dashboard - Edge Video V2
Write-Host "========================================"
Write-Host "  Atualizando Dashboard"
Write-Host "========================================"
Write-Host ""

$cred = [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes("admin:admin"))
$headers = @{
    Authorization = "Basic $cred"
    "Content-Type" = "application/json"
}

Write-Host "[1/3] Obtendo dashboard funcional..." -ForegroundColor Yellow
try {
    $fixedDashboard = Invoke-RestMethod -Uri "http://localhost:3000/api/dashboards/uid/edge-video-v2-fixed" -Headers $headers -Method Get
    Write-Host "  OK - Dashboard encontrado" -ForegroundColor Green
} catch {
    Write-Host "  ERRO - Dashboard nao encontrado" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "[2/3] Atualizando configuracoes..." -ForegroundColor Yellow
$db = $fixedDashboard.dashboard
$db.uid = "edge-video-v2"
$db.title = "Edge Video V2 - Computer Vision Monitoring"
$db.id = $null
$db.version = $null
Write-Host "  OK - Configuracoes atualizadas" -ForegroundColor Green

Write-Host ""
Write-Host "[3/3] Importando para Grafana..." -ForegroundColor Yellow
$body = @{
    dashboard = $db
    overwrite = $true
    message = "Dashboard atualizado via script"
} | ConvertTo-Json -Depth 30

try {
    $result = Invoke-RestMethod -Uri "http://localhost:3000/api/dashboards/db" -Headers $headers -Method Post -Body $body
    Write-Host "  OK - Dashboard importado" -ForegroundColor Green
} catch {
    Write-Host "  ERRO ao importar" -ForegroundColor Red
    Write-Host $_.Exception.Message
    exit 1
}

Write-Host ""
Write-Host "========================================"
Write-Host "  DASHBOARD ATUALIZADO COM SUCESSO!"
Write-Host "========================================"
Write-Host ""
Write-Host "Acesse: http://localhost:3000$($result.url)" -ForegroundColor Cyan
Write-Host ""
