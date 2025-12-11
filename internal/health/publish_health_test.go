package health

import (
	"sync"
	"testing"
	"time"
)

// TestNewPublishHealthMonitor validates monitor creation with correct configuration
func TestNewPublishHealthMonitor(t *testing.T) {
	monitor := NewPublishHealthMonitor(10, 0.8)

	if monitor == nil {
		t.Fatal("Expected monitor to be created, got nil")
	}

	if monitor.windowSize != 10 {
		t.Errorf("Expected windowSize 10, got %d", monitor.windowSize)
	}

	if monitor.degradedThreshold != 0.8 {
		t.Errorf("Expected degradedThreshold 0.8, got %f", monitor.degradedThreshold)
	}

	if len(monitor.recentErrors) != 0 {
		t.Errorf("Expected empty recentErrors, got %d elements", len(monitor.recentErrors))
	}

	if len(monitor.recentSuccesses) != 0 {
		t.Errorf("Expected empty recentSuccesses, got %d elements", len(monitor.recentSuccesses))
	}
}

// TestNewPublishHealthMonitorDefaults validates default values
func TestNewPublishHealthMonitorDefaults(t *testing.T) {
	// Test with invalid values - should use defaults
	monitor := NewPublishHealthMonitor(0, 0)

	if monitor.windowSize != 10 {
		t.Errorf("Expected default windowSize 10, got %d", monitor.windowSize)
	}

	if monitor.degradedThreshold != 0.8 {
		t.Errorf("Expected default degradedThreshold 0.8, got %f", monitor.degradedThreshold)
	}
}

// TestRecordSuccessAndError validates success and error recording
func TestRecordSuccessAndError(t *testing.T) {
	monitor := NewPublishHealthMonitor(10, 0.8)

	// Record 3 successes
	monitor.RecordSuccess()
	monitor.RecordSuccess()
	monitor.RecordSuccess()

	stats := monitor.GetStats()

	if stats.RecentSuccesses != 3 {
		t.Errorf("Expected 3 successes, got %d", stats.RecentSuccesses)
	}

	if stats.RecentErrors != 0 {
		t.Errorf("Expected 0 errors, got %d", stats.RecentErrors)
	}

	if stats.ConsecutiveFails != 0 {
		t.Errorf("Expected 0 consecutive fails, got %d", stats.ConsecutiveFails)
	}

	// Record 2 errors
	monitor.RecordError()
	monitor.RecordError()

	stats = monitor.GetStats()

	if stats.RecentSuccesses != 3 {
		t.Errorf("Expected 3 successes, got %d", stats.RecentSuccesses)
	}

	if stats.RecentErrors != 2 {
		t.Errorf("Expected 2 errors, got %d", stats.RecentErrors)
	}

	if stats.ConsecutiveFails != 2 {
		t.Errorf("Expected 2 consecutive fails, got %d", stats.ConsecutiveFails)
	}

	// Record success - should reset consecutive fails
	monitor.RecordSuccess()

	stats = monitor.GetStats()

	if stats.ConsecutiveFails != 0 {
		t.Errorf("Expected consecutive fails to reset to 0, got %d", stats.ConsecutiveFails)
	}
}

// TestSlidingWindow validates sliding window behavior
func TestSlidingWindow(t *testing.T) {
	monitor := NewPublishHealthMonitor(5, 0.8) // Small window for testing

	// Fill window completely (5 successes)
	for i := 0; i < 5; i++ {
		monitor.RecordSuccess()
	}

	stats := monitor.GetStats()

	if stats.RecentSuccesses != 5 {
		t.Errorf("Expected 5 successes, got %d", stats.RecentSuccesses)
	}

	// Add many more elements - window should not exceed windowSize
	for i := 0; i < 10; i++ {
		monitor.RecordError()
	}

	stats = monitor.GetStats()

	// Window should never exceed windowSize
	if len(monitor.recentErrors) > monitor.windowSize {
		t.Errorf("Expected errors window ≤ %d, got %d", monitor.windowSize, len(monitor.recentErrors))
	}

	if len(monitor.recentSuccesses) > monitor.windowSize {
		t.Errorf("Expected successes window ≤ %d, got %d", monitor.windowSize, len(monitor.recentSuccesses))
	}

	// Add more successes
	for i := 0; i < 10; i++ {
		monitor.RecordSuccess()
	}

	// Verify windows still bounded
	if len(monitor.recentErrors) > monitor.windowSize {
		t.Errorf("Expected errors window ≤ %d, got %d", monitor.windowSize, len(monitor.recentErrors))
	}

	if len(monitor.recentSuccesses) > monitor.windowSize {
		t.Errorf("Expected successes window ≤ %d, got %d", monitor.windowSize, len(monitor.recentSuccesses))
	}

	t.Log("✅ Sliding window properly bounded")
}

// TestIsDegradedByErrorRate validates degradation detection by error rate
func TestIsDegradedByErrorRate(t *testing.T) {
	monitor := NewPublishHealthMonitor(10, 0.8)

	// Fill window: 8 errors, 2 successes = 80% error rate
	for i := 0; i < 8; i++ {
		monitor.RecordError()
	}

	for i := 0; i < 2; i++ {
		monitor.RecordSuccess()
	}

	if !monitor.IsDegraded() {
		t.Error("Expected IsDegraded to be true with 80% error rate")
	}

	stats := monitor.GetStats()

	if stats.ErrorRate != 0.8 {
		t.Errorf("Expected error rate 0.8, got %f", stats.ErrorRate)
	}

	if !stats.IsDegraded {
		t.Error("Expected stats.IsDegraded to be true")
	}
}

// TestIsDegradedByTimeout validates degradation detection by timeout (no success in 30s)
func TestIsDegradedByTimeout(t *testing.T) {
	monitor := NewPublishHealthMonitor(10, 0.8)

	// Fill window first (isDegradedUnsafe requires window to be full)
	for i := 0; i < 10; i++ {
		monitor.RecordSuccess()
	}

	// Set lastSuccess to 31 seconds ago
	monitor.mu.Lock()
	monitor.lastSuccess = time.Now().Add(-31 * time.Second)
	monitor.mu.Unlock()

	if !monitor.IsDegraded() {
		t.Error("Expected IsDegraded to be true after 30s timeout")
	}

	stats := monitor.GetStats()

	if !stats.IsDegraded {
		t.Error("Expected stats.IsDegraded to be true")
	}
}

// TestIsEmergency validates emergency state detection (>20 consecutive failures)
func TestIsEmergency(t *testing.T) {
	monitor := NewPublishHealthMonitor(10, 0.8)

	// Record 21 consecutive failures
	for i := 0; i < 21; i++ {
		monitor.RecordError()
	}

	if !monitor.IsEmergency() {
		t.Error("Expected IsEmergency to be true after 21 failures")
	}

	stats := monitor.GetStats()

	if stats.ConsecutiveFails != 21 {
		t.Errorf("Expected 21 consecutive fails, got %d", stats.ConsecutiveFails)
	}

	if !stats.IsEmergency {
		t.Error("Expected stats.IsEmergency to be true")
	}

	// Test boundary: 20 failures should NOT be emergency
	monitor2 := NewPublishHealthMonitor(10, 0.8)

	for i := 0; i < 20; i++ {
		monitor2.RecordError()
	}

	if monitor2.IsEmergency() {
		t.Error("Expected IsEmergency to be false with exactly 20 failures")
	}
}

// TestNoDeadlock verifies that GetStats() does NOT cause deadlock (PR#2 fix)
// This was the critical bug that froze the producer for 14 hours
func TestNoDeadlock(t *testing.T) {
	monitor := NewPublishHealthMonitor(10, 0.8)

	done := make(chan bool)
	iterations := 1000

	// Goroutine 1: Record success continuously
	go func() {
		for i := 0; i < iterations; i++ {
			monitor.RecordSuccess()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Goroutine 2: Record error continuously
	go func() {
		for i := 0; i < iterations; i++ {
			monitor.RecordError()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Goroutine 3: Get stats continuously (this used to deadlock!)
	go func() {
		for i := 0; i < iterations; i++ {
			_ = monitor.GetStats()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Should complete without deadlock
	select {
	case <-done:
		// Success - no deadlock!
		t.Log("✅ No deadlock detected - test passed")
	case <-time.After(10 * time.Second):
		t.Fatal("❌ DEADLOCK DETECTED! GetStats() blocked for >10s")
	}
}

// TestConcurrentAccess validates thread-safety under high concurrency
func TestConcurrentAccess(t *testing.T) {
	monitor := NewPublishHealthMonitor(100, 0.8)

	var wg sync.WaitGroup
	goroutines := 50
	operationsPerGoroutine := 100

	// Launch many goroutines doing concurrent operations
	for i := 0; i < goroutines; i++ {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				if id%3 == 0 {
					monitor.RecordSuccess()
				} else if id%3 == 1 {
					monitor.RecordError()
				} else {
					_ = monitor.GetStats()
				}
			}
		}(i)
	}

	// Wait with timeout
	doneChan := make(chan bool)

	go func() {
		wg.Wait()
		doneChan <- true
	}()

	select {
	case <-doneChan:
		t.Log("✅ Concurrent access test passed")
	case <-time.After(15 * time.Second):
		t.Fatal("❌ Concurrent access test timed out - possible deadlock")
	}

	// Verify final state is valid
	stats := monitor.GetStats()

	if stats.TotalRecent < 0 {
		t.Errorf("Invalid total: %d", stats.TotalRecent)
	}

	if stats.ErrorRate < 0 || stats.ErrorRate > 1.0 {
		t.Errorf("Invalid error rate: %f", stats.ErrorRate)
	}
}

// TestReset validates that Reset() clears all statistics
func TestReset(t *testing.T) {
	monitor := NewPublishHealthMonitor(10, 0.8)

	// Add some data
	for i := 0; i < 5; i++ {
		monitor.RecordSuccess()
		monitor.RecordError()
	}

	stats := monitor.GetStats()

	if stats.TotalRecent != 10 {
		t.Errorf("Expected 10 total before reset, got %d", stats.TotalRecent)
	}

	// Reset
	monitor.Reset()

	stats = monitor.GetStats()

	if stats.TotalRecent != 0 {
		t.Errorf("Expected 0 total after reset, got %d", stats.TotalRecent)
	}

	if stats.RecentErrors != 0 {
		t.Errorf("Expected 0 errors after reset, got %d", stats.RecentErrors)
	}

	if stats.RecentSuccesses != 0 {
		t.Errorf("Expected 0 successes after reset, got %d", stats.RecentSuccesses)
	}

	if stats.ConsecutiveFails != 0 {
		t.Errorf("Expected 0 consecutive fails after reset, got %d", stats.ConsecutiveFails)
	}
}

// BenchmarkGetStats measures GetStats() performance
func BenchmarkGetStats(b *testing.B) {
	monitor := NewPublishHealthMonitor(10, 0.8)

	// Fill with data
	for i := 0; i < 10; i++ {
		monitor.RecordSuccess()
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = monitor.GetStats()
	}
}

// BenchmarkRecordSuccess measures RecordSuccess() performance
func BenchmarkRecordSuccess(b *testing.B) {
	monitor := NewPublishHealthMonitor(100, 0.8)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		monitor.RecordSuccess()
	}
}

// BenchmarkConcurrentGetStats measures concurrent GetStats() performance
func BenchmarkConcurrentGetStats(b *testing.B) {
	monitor := NewPublishHealthMonitor(10, 0.8)

	// Fill with data
	for i := 0; i < 10; i++ {
		monitor.RecordSuccess()
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = monitor.GetStats()
		}
	})
}
