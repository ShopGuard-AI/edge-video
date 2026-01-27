package capabilities

import (
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	hasQSV     bool
	hasChecked bool
	checkMu    sync.Mutex
)

// HasQSV verifica se o sistema tem suporte a Intel QuickSync (apenas uma vez)
func HasQSV() bool {
	checkMu.Lock()
	defer checkMu.Unlock()

	if hasChecked {
		return hasQSV
	}

	hasQSV = checkFFmpegQSV()
	hasChecked = true
	return hasQSV
}

func checkFFmpegQSV() bool {
	// Executa "ffmpeg -hwaccels" com timeout
	ctx := time.After(2 * time.Second) // Timeout curto
	_ = ctx

	cmd := exec.Command("ffmpeg", "-hwaccels")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[Capabilities] Erro ao checar FFmpeg: %v", err)
		return false
	}

	// Verifica se "qsv" ou "d3d11va" está na saída
	// d3d11va também é um bom indicador de fallback para Intel/NVIDIA no Windows
	outStr := strings.ToLower(string(output))
	if strings.Contains(outStr, "qsv") {
		log.Println("[Capabilities] ✓ Suporte a Intel QuickSync (QSV) detectado.")
		return true
	}

	log.Println("[Capabilities] Hardware acceleration QSV não detectado (usando CPU).")
	return false
}

var (
	hasMJPEGQSV  bool
	checkedMJPEG bool
	mjpegMu      sync.Mutex
)

// HasMJPEGQSV verifica se existe ENCODER mjpeg_qsv
func HasMJPEGQSV() bool {
	mjpegMu.Lock()
	defer mjpegMu.Unlock()

	if checkedMJPEG {
		return hasMJPEGQSV
	}

	hasMJPEGQSV = checkEncoderMJPEGQSV()
	checkedMJPEG = true
	return hasMJPEGQSV
}

func checkEncoderMJPEGQSV() bool {
	ctx := time.After(2 * time.Second)
	_ = ctx

	// Verifica encoders
	cmd := exec.Command("ffmpeg", "-encoders")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	if strings.Contains(string(output), "mjpeg_qsv") {
		log.Println("[Capabilities] ✓ Encoder MJPEG via Hardware (mjpeg_qsv) detectado.")
		return true
	}

	log.Println("[Capabilities] Encoder MJPEG QSV não encontrado. Usando Software Encoder.")
	return false
}
