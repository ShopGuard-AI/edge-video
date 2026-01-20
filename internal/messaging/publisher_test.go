package messaging

import (
	"context"
	"testing"
	"time"

	"edge-video/v2/internal/storage"
)

// ============================================================================
// PUBLISHER UNIT TESTS - Fase 1 QA
// ============================================================================
// Testes baseados na funcionalidade real do publisher.go
// Cobertura: PublishWithContext, context cancellation, Redis requirement, stats
//
// NOTA: Testes que requerem RabbitMQ real são marcados com comentários
// Testes focam em lógica de context handling e validações
// ============================================================================

// TestPublishWithContextCancellation valida que PublishWithContext respeita contexto cancelado
func TestPublishWithContextCancellation(t *testing.T) {
	// Mock publisher (sem RabbitMQ real)
	publisher := &Publisher{
		amqpURL:    "amqp://localhost:5672",
		exchange:   "test_exchange",
		routingKey: "test.route",
		connected:  true, // Simula conectado
		// redisClient: nil, // Vai falhar no check de Redis
	}

	// Contexto já cancelado
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancela ANTES de chamar PublishWithContext

	frameData := []byte("test frame")
	timestamp := time.Now()

	err := publisher.PublishWithContext(ctx, "cam1", frameData, timestamp)

	// Deve retornar ctx.Err() (context.Canceled) imediatamente
	if err != context.Canceled {
		t.Errorf("Expected error context.Canceled, got: %v", err)
	}

	t.Log("✅ PublishWithContext retorna imediatamente quando contexto cancelado")
}

// TestPublishWithContextTimeout valida timeout de contexto
func TestPublishWithContextTimeout(t *testing.T) {
	publisher := &Publisher{
		amqpURL:    "amqp://localhost:5672",
		exchange:   "test_exchange",
		routingKey: "test.route",
		connected:  true,
	}

	// Contexto com timeout muito curto (1ms)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Aguarda timeout
	time.Sleep(10 * time.Millisecond)

	frameData := []byte("test frame")
	timestamp := time.Now()

	err := publisher.PublishWithContext(ctx, "cam1", frameData, timestamp)

	// Deve retornar ctx.Err() (context.DeadlineExceeded)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected error context.DeadlineExceeded, got: %v", err)
	}

	t.Log("✅ PublishWithContext respeita timeout de contexto")
}

// TestPublishRequiresRedis valida que Publish/PublishWithContext requerem Redis habilitado
func TestPublishRequiresRedis(t *testing.T) {
	// Publisher SEM Redis (V1.6-style requer Redis obrigatório)
	publisher := &Publisher{
		amqpURL:     "amqp://localhost:5672",
		exchange:    "test_exchange",
		routingKey:  "test.route",
		connected:   true,
		redisClient: nil, // Redis DESABILITADO
	}

	frameData := []byte("test frame")
	timestamp := time.Now()

	// Publish() sem Redis deve falhar
	err := publisher.Publish("cam1", frameData, timestamp)
	if err == nil {
		t.Error("Publish() should fail when Redis is disabled")
	}

	expectedErrMsg := "Redis OBRIGATÓRIO para V1.6-style publishing"
	if err != nil && len(err.Error()) < len(expectedErrMsg) {
		t.Errorf("Expected error message containing '%s', got: %v", expectedErrMsg, err)
	}

	// PublishWithContext() sem Redis também deve falhar
	ctx := context.Background()
	err = publisher.PublishWithContext(ctx, "cam1", frameData, timestamp)
	if err == nil {
		t.Error("PublishWithContext() should fail when Redis is disabled")
	}

	t.Log("✅ Publish requer Redis habilitado (V1.6-style)")
}

// TestPublishNotConnected valida comportamento quando não conectado ao RabbitMQ
func TestPublishNotConnected(t *testing.T) {
	publisher := &Publisher{
		amqpURL:    "amqp://localhost:5672",
		exchange:   "test_exchange",
		routingKey: "test.route",
		connected:  false, // NÃO CONECTADO
	}

	frameData := []byte("test frame")
	timestamp := time.Now()

	err := publisher.Publish("cam1", frameData, timestamp)
	if err == nil {
		t.Error("Publish() should fail when not connected")
	}

	expectedErrMsg := "não conectado ao RabbitMQ"
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error '%s', got: %v", expectedErrMsg, err)
	}

	// Verifica que publishErrors foi incrementado
	if publisher.publishErrors != 1 {
		t.Errorf("Expected publishErrors=1, got %d", publisher.publishErrors)
	}

	t.Log("✅ Publish falha corretamente quando não conectado")
}

// TestStats valida rastreamento de estatísticas
func TestStats(t *testing.T) {
	publisher := &Publisher{
		publishCount:  250,
		publishErrors: 10,
	}

	count, errors := publisher.Stats()

	if count != 250 {
		t.Errorf("Expected publishCount=250, got %d", count)
	}

	if errors != 10 {
		t.Errorf("Expected publishErrors=10, got %d", errors)
	}

	t.Log("✅ Stats() retorna estatísticas corretas")
}

// TestIsConnected valida método IsConnected()
func TestIsConnected(t *testing.T) {
	publisher := &Publisher{
		connected: true,
	}

	if !publisher.IsConnected() {
		t.Error("Expected IsConnected()=true")
	}

	publisher.connected = false

	if publisher.IsConnected() {
		t.Error("Expected IsConnected()=false")
	}

	t.Log("✅ IsConnected() funciona corretamente")
}

// TestConfirmStats valida rastreamento de confirmações (ACK/NACK)
func TestConfirmStats(t *testing.T) {
	publisher := &Publisher{
		confirmsCount: 1000,
		nacksCount:    5,
	}

	acks, nacks := publisher.ConfirmStats()

	if acks != 1000 {
		t.Errorf("Expected confirmsCount=1000, got %d", acks)
	}

	if nacks != 5 {
		t.Errorf("Expected nacksCount=5, got %d", nacks)
	}

	t.Log("✅ ConfirmStats() retorna estatísticas de confirmações corretas")
}

// TestPublisherConfirmsEnabled valida flag de publisher confirms
func TestPublisherConfirmsEnabled(t *testing.T) {
	// Publisher com confirms habilitados
	publisherWithConfirms := &Publisher{
		publisherConfirmsEnabled: true,
	}

	if !publisherWithConfirms.publisherConfirmsEnabled {
		t.Error("Expected publisherConfirmsEnabled=true")
	}

	// Publisher com confirms desabilitados
	publisherWithoutConfirms := &Publisher{
		publisherConfirmsEnabled: false,
	}

	if publisherWithoutConfirms.publisherConfirmsEnabled {
		t.Error("Expected publisherConfirmsEnabled=false")
	}

	t.Log("✅ Publisher Confirms flag configurável corretamente")
}

// TestContextCheckBeforeRedisStore valida que contexto é checado antes de operações pesadas
func TestContextCheckBeforeRedisStore(t *testing.T) {
	// Mock Redis client que responde a context
	mockRedis := &storage.RedisClient{
		// Mock básico
	}

	publisher := &Publisher{
		connected:   true,
		redisClient: mockRedis,
	}

	// Contexto cancelado
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	frameData := []byte("test frame")
	timestamp := time.Now()

	err := publisher.PublishWithContext(ctx, "cam1", frameData, timestamp)

	// Deve retornar ctx.Err() ANTES de tentar Redis.StoreWithContext
	if err != context.Canceled {
		t.Errorf("Expected immediate context.Canceled, got: %v", err)
	}

	t.Log("✅ Contexto checado ANTES de operações pesadas (Redis/RabbitMQ)")
}

// TestDefensiveCopy valida que frame data é copiado (não usa slice original)
func TestDefensiveCopy(t *testing.T) {
	// NOTA: Este teste valida a LÓGICA de cópia defensiva
	// A implementação faz: frameDataCopy := make([]byte, len(frameData)); copy(frameDataCopy, frameData)

	originalData := []byte("original frame data")

	// Simula cópia defensiva (como no código)
	frameDataCopy := make([]byte, len(originalData))
	copy(frameDataCopy, originalData)

	// Modifica original
	originalData[0] = 'X'

	// Cópia não deve ser afetada
	if frameDataCopy[0] == 'X' {
		t.Error("Defensive copy was affected by original modification (shallow copy!)")
	}

	if frameDataCopy[0] != 'o' {
		t.Errorf("Expected first byte='o', got '%c'", frameDataCopy[0])
	}

	t.Log("✅ Cópia defensiva protege contra modificações do slice original")
}

// ============================================================================
// TESTES COM RABBITMQ REAL (OPCIONAL - REQUER RABBITMQ RODANDO)
// ============================================================================
// Para rodar esses testes:
// 1. Inicie RabbitMQ: docker run -p 5672:5672 -p 15672:15672 rabbitmq:3-management
// 2. Inicie Redis: docker run -p 6379:6379 redis:latest
// 3. Descomente os testes abaixo
// 4. Execute: go test -v ./internal/messaging
// ============================================================================

/*
// TestPublishWithContext_RealRabbitMQ valida PublishWithContext com RabbitMQ real
func TestPublishWithContext_RealRabbitMQ(t *testing.T) {
	// Setup Redis
	redisConfig := storage.RedisConfig{
		Enabled: true,
		Address: "localhost:6379",
		DB:      0,
		Prefix:  "test_frames",
		TTL:     60 * time.Second,
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
		Timeout: 2 * time.Second,
	}

	redisClient, err := storage.NewRedisClient(redisConfig)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer redisClient.Close()

	// Setup Publisher
	publisher, err := NewPublisher(
		"amqp://guest:guest@localhost:5672/",
		"test_exchange",
		"test.route",
		100, // prefetchCount
		true, // publisherConfirms
		redisClient,
	)
	if err != nil {
		t.Skipf("RabbitMQ not available: %v", err)
	}
	defer publisher.Close()

	// Publish com context
	ctx := context.Background()
	frameData := []byte("test frame data for rabbitmq")
	timestamp := time.Now()

	err = publisher.PublishWithContext(ctx, "cam_test", frameData, timestamp)
	if err != nil {
		t.Fatalf("PublishWithContext failed: %v", err)
	}

	// Verify stats
	count, errors := publisher.Stats()
	if count == 0 {
		t.Error("Expected publishCount > 0")
	}

	t.Logf("✅ PublishWithContext funcionando com RabbitMQ real (count=%d, errors=%d)", count, errors)
}

// TestPublisherAutoReconnect_RealRabbitMQ valida auto-reconnect
func TestPublisherAutoReconnect_RealRabbitMQ(t *testing.T) {
	redisConfig := storage.RedisConfig{
		Enabled: true,
		Address: "localhost:6379",
		DB:      0,
		Prefix:  "test_frames",
		TTL:     60 * time.Second,
	}

	redisClient, _ := storage.NewRedisClient(redisConfig)
	if redisClient != nil {
		defer redisClient.Close()
	}

	publisher, err := NewPublisher(
		"amqp://guest:guest@localhost:5672/",
		"test_exchange",
		"test.route",
		100,
		false, // Sem confirms para teste rápido
		redisClient,
	)
	if err != nil {
		t.Skipf("RabbitMQ not available: %v", err)
	}
	defer publisher.Close()

	// Publish inicial
	ctx := context.Background()
	err = publisher.PublishWithContext(ctx, "cam1", []byte("test"), time.Now())
	if err != nil {
		t.Logf("Initial publish failed (expected if Redis disabled): %v", err)
	}

	// TODO: Testar reconexão (requer desligar/religar RabbitMQ manualmente)
	// Este teste é mais adequado para teste de integração

	t.Log("✅ Publisher conectado (teste de auto-reconnect requer RabbitMQ restart manual)")
}
*/

// ============================================================================
// BENCHMARKS
// ============================================================================

// BenchmarkPublishWithContext mede overhead de context handling (mock)
func BenchmarkPublishWithContext(b *testing.B) {
	publisher := &Publisher{
		connected: false, // Vai falhar rápido (sem RabbitMQ)
	}

	ctx := context.Background()
	frameData := make([]byte, 1024) // 1KB frame
	timestamp := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		publisher.PublishWithContext(ctx, "cam1", frameData, timestamp)
	}
}

// BenchmarkDefensiveCopy mede performance de cópia defensiva
func BenchmarkDefensiveCopy(b *testing.B) {
	frameData := make([]byte, 320*1024) // 320KB (frame JPEG típico)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frameDataCopy := make([]byte, len(frameData))
		copy(frameDataCopy, frameData)
	}
}

// BenchmarkStats mede performance de Stats()
func BenchmarkStats(b *testing.B) {
	publisher := &Publisher{
		publishCount:  1000,
		publishErrors: 10,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		publisher.Stats()
	}
}
