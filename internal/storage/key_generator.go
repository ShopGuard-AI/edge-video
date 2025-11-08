package storage

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// KeyStrategy define a estratégia de geração de chaves Redis
type KeyStrategy string

const (
	// StrategyBasic usa apenas timestamp (não recomendado para alta concorrência)
	StrategyBasic KeyStrategy = "basic"
	// StrategySequence usa timestamp + contador sequencial (recomendado)
	StrategySequence KeyStrategy = "sequence"
	// StrategyUUID usa timestamp + UUID (para sistemas distribuídos)
	StrategyUUID KeyStrategy = "uuid"
)

// KeyGeneratorConfig configuração do gerador de chaves
type KeyGeneratorConfig struct {
	Strategy KeyStrategy
	Prefix   string
	Vhost    string // Identificador único do cliente (extraído do AMQP vhost)
}

// KeyGenerator gera chaves únicas para frames no Redis
type KeyGenerator struct {
	config   KeyGeneratorConfig
	sequence uint64
	mu       sync.RWMutex
}

// NewKeyGenerator cria um novo gerador de chaves
func NewKeyGenerator(config KeyGeneratorConfig) *KeyGenerator {
	// Usa sequence como estratégia padrão se não especificado
	if config.Strategy == "" {
		config.Strategy = StrategySequence
	}
	
	// Usa "default" como vhost se não especificado
	if config.Vhost == "" {
		config.Vhost = "default"
	}

	return &KeyGenerator{
		config: config,
	}
}

// GenerateKey gera uma chave única para um frame
// Formato: {vhost}:{prefix}:{cameraID}:{unix_timestamp}:{sufixo}
// Exemplo: supermercado_vhost:frames:cam4:1731024000123456789:00001
func (kg *KeyGenerator) GenerateKey(cameraID string, timestamp time.Time) string {
	baseKey := fmt.Sprintf("%s:%s:%s:%d",
		kg.config.Vhost,
		kg.config.Prefix,
		cameraID,
		timestamp.UnixNano(),
	)

	switch kg.config.Strategy {
	case StrategySequence:
		seq := kg.getNextSequence()
		return fmt.Sprintf("%s:%05d", baseKey, seq)
	case StrategyUUID:
		return fmt.Sprintf("%s:%s", baseKey, uuid.New().String()[:8])
	case StrategyBasic:
		fallthrough
	default:
		return baseKey
	}
}

// ParseKey decompõe uma chave Redis em seus componentes
type KeyComponents struct {
	Prefix    string
	Vhost     string
	CameraID  string
	Timestamp time.Time
	Suffix    string
}

// ParseKey extrai os componentes de uma chave Redis
func (kg *KeyGenerator) ParseKey(key string) (*KeyComponents, error) {
	// Formato: vhost:prefix:cameraID:unix_timestamp[:suffix]
	
	// Encontra os primeiros 3 ":" para separar vhost:prefix:cameraID
	firstColon := strings.Index(key, ":")
	if firstColon == -1 {
		return nil, fmt.Errorf("invalid key format: %s", key)
	}
	vhost := key[:firstColon]
	
	secondColon := strings.Index(key[firstColon+1:], ":")
	if secondColon == -1 {
		return nil, fmt.Errorf("invalid key format: %s", key)
	}
	secondColon += firstColon + 1
	prefix := key[firstColon+1 : secondColon]
	
	thirdColon := strings.Index(key[secondColon+1:], ":")
	if thirdColon == -1 {
		return nil, fmt.Errorf("invalid key format: %s", key)
	}
	thirdColon += secondColon + 1
	cameraID := key[secondColon+1 : thirdColon]
	
	// O resto é unix_timestamp[:suffix]
	remaining := key[thirdColon+1:]
	
	// Procura por sufixo (após o último ":")
	lastColon := strings.LastIndex(remaining, ":")
	var timestampStr, suffix string
	
	if lastColon > 0 {
		timestampStr = remaining[:lastColon]
		suffix = remaining[lastColon+1:]
	} else {
		timestampStr = remaining
	}
	
	// Parse Unix timestamp (em nanoseconds)
	var unixNano int64
	_, err := fmt.Sscanf(timestampStr, "%d", &unixNano)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp in key: %w", err)
	}
	
	timestamp := time.Unix(0, unixNano)
	
	return &KeyComponents{
		Prefix:    prefix,
		Vhost:     vhost,
		CameraID:  cameraID,
		Timestamp: timestamp,
		Suffix:    suffix,
	}, nil
}

// QueryPattern retorna o padrão para buscar chaves no Redis
// Exemplos:
// - Todas as chaves de um vhost: QueryPattern("", "")
// - Todas as chaves de uma câmera: QueryPattern("cam1", "")
func (kg *KeyGenerator) QueryPattern(cameraID string, vhost string) string {
	// Usa o vhost configurado se não for especificado
	if vhost == "" {
		vhost = kg.config.Vhost
	}

	if cameraID == "" {
		return fmt.Sprintf("%s:%s:*", vhost, kg.config.Prefix)
	}
	return fmt.Sprintf("%s:%s:%s:*", vhost, kg.config.Prefix, cameraID)
}

// getNextSequence retorna o próximo número sequencial (thread-safe)
func (kg *KeyGenerator) getNextSequence() uint64 {
	kg.mu.Lock()
	defer kg.mu.Unlock()
	kg.sequence++
	// Reset após 99999 para manter o formato de 5 dígitos
	if kg.sequence > 99999 {
		kg.sequence = 1
	}
	return kg.sequence
}

// GetConfig retorna a configuração atual
func (kg *KeyGenerator) GetConfig() KeyGeneratorConfig {
	return kg.config
}
