# Edge Video V2 - Setup Script
# Baixa e configura FFmpeg automaticamente

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Edge Video V2 - Setup Automático" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 1. Verificar se FFmpeg já existe
if (Test-Path ".\ffmpeg.exe") {
    Write-Host "✓ FFmpeg já está instalado!" -ForegroundColor Green
    & .\ffmpeg.exe -version | Select-Object -First 1
    Write-Host ""
    $response = Read-Host "Deseja reinstalar? (s/N)"
    if ($response -ne "s" -and $response -ne "S") {
        Write-Host "Setup concluído!" -ForegroundColor Green
        exit 0
    }
    Remove-Item ".\ffmpeg.exe" -Force
}

# 2. Baixar FFmpeg
Write-Host "[1/4] Baixando FFmpeg..." -ForegroundColor Yellow

$ffmpegUrl = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
$zipFile = "ffmpeg-temp.zip"

try {
    # Mostra progresso do download
    $ProgressPreference = 'SilentlyContinue'
    Invoke-WebRequest -Uri $ffmpegUrl -OutFile $zipFile -UseBasicParsing
    $ProgressPreference = 'Continue'

    $zipSize = (Get-Item $zipFile).Length / 1MB
    Write-Host "  ✓ Download completo: $([math]::Round($zipSize, 1)) MB" -ForegroundColor Green
} catch {
    Write-Host "  ✗ Erro ao baixar FFmpeg!" -ForegroundColor Red
    Write-Host "  Baixe manualmente de: https://github.com/BtbN/FFmpeg-Builds/releases" -ForegroundColor Yellow
    exit 1
}

# 3. Extrair FFmpeg
Write-Host "[2/4] Extraindo FFmpeg..." -ForegroundColor Yellow

try {
    Expand-Archive -Path $zipFile -DestinationPath "." -Force

    # Encontrar o diretório extraído (nome varia)
    $extractedDir = Get-ChildItem -Directory | Where-Object { $_.Name -like "ffmpeg-*" } | Select-Object -First 1

    if ($extractedDir) {
        Write-Host "  ✓ Arquivos extraídos" -ForegroundColor Green
    } else {
        throw "Diretório FFmpeg não encontrado após extração"
    }
} catch {
    Write-Host "  ✗ Erro ao extrair arquivos!" -ForegroundColor Red
    Write-Host "  Erro: $_" -ForegroundColor Red
    exit 1
}

# 4. Copiar binário
Write-Host "[3/4] Instalando FFmpeg..." -ForegroundColor Yellow

try {
    $ffmpegBinary = Join-Path $extractedDir.FullName "bin\ffmpeg.exe"

    if (Test-Path $ffmpegBinary) {
        Copy-Item $ffmpegBinary -Destination ".\ffmpeg.exe" -Force
        Write-Host "  ✓ FFmpeg copiado para diretório atual" -ForegroundColor Green
    } else {
        throw "ffmpeg.exe não encontrado em $ffmpegBinary"
    }
} catch {
    Write-Host "  ✗ Erro ao copiar FFmpeg!" -ForegroundColor Red
    Write-Host "  Erro: $_" -ForegroundColor Red
    exit 1
}

# 5. Limpar arquivos temporários
Write-Host "[4/4] Limpando arquivos temporários..." -ForegroundColor Yellow

try {
    Remove-Item $zipFile -Force -ErrorAction SilentlyContinue
    Remove-Item $extractedDir.FullName -Recurse -Force -ErrorAction SilentlyContinue
    Write-Host "  ✓ Arquivos temporários removidos" -ForegroundColor Green
} catch {
    Write-Host "  ⚠ Aviso: Não foi possível remover alguns arquivos temporários" -ForegroundColor Yellow
}

# 6. Validar instalação
Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Validando instalação..." -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

try {
    $ffmpegVersion = & .\ffmpeg.exe -version 2>&1 | Select-Object -First 1
    Write-Host ""
    Write-Host "✓ FFmpeg instalado com sucesso!" -ForegroundColor Green
    Write-Host "  Versão: $ffmpegVersion" -ForegroundColor Gray
    Write-Host ""
} catch {
    Write-Host "✗ Erro ao validar FFmpeg!" -ForegroundColor Red
    Write-Host "  Tente executar .\ffmpeg.exe -version manualmente" -ForegroundColor Yellow
    exit 1
}

# 7. Próximos passos
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Próximos Passos" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "1. Edite config.yaml com suas configurações" -ForegroundColor White
Write-Host "   - Redis (IP, senha)" -ForegroundColor Gray
Write-Host "   - RabbitMQ (URL, credenciais)" -ForegroundColor Gray
Write-Host "   - Câmeras (URLs, IDs)" -ForegroundColor Gray
Write-Host ""
Write-Host "2. Execute o producer:" -ForegroundColor White
Write-Host "   .\producer.exe" -ForegroundColor Yellow
Write-Host ""
Write-Host "3. Para instalar como serviço Windows:" -ForegroundColor White
Write-Host "   Consulte DEPLOY_WINDOWS.md" -ForegroundColor Gray
Write-Host ""
Write-Host "✓ Setup concluído!" -ForegroundColor Green
Write-Host ""
