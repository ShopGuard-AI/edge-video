package circuit

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewBreaker(t *testing.T) {
	breaker := NewBreaker("test", 3, 10*time.Second)
	
	assert.NotNil(t, breaker)
	assert.Equal(t, "test", breaker.name)
	assert.Equal(t, int64(3), breaker.maxFailures)
	assert.Equal(t, 10*time.Second, breaker.resetTimeout)
	assert.Equal(t, StateClosed, breaker.State())
}

func TestBreakerStateClosed(t *testing.T) {
	breaker := NewBreaker("test", 3, 1*time.Second)
	
	assert.Equal(t, StateClosed, breaker.State())
	
	err := breaker.Call(func() error {
		return nil
	})
	
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, breaker.State())
}

func TestBreakerStateOpen(t *testing.T) {
	breaker := NewBreaker("test", 3, 1*time.Second)
	
	for i := 0; i < 3; i++ {
		_ = breaker.Call(func() error {
			return errors.New("test error")
		})
	}
	
	assert.Equal(t, StateOpen, breaker.State())
	
	err := breaker.Call(func() error {
		return nil
	})
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker")
}

func TestBreakerStateHalfOpen(t *testing.T) {
	breaker := NewBreaker("test", 2, 100*time.Millisecond)
	
	_ = breaker.Call(func() error {
		return errors.New("error 1")
	})
	_ = breaker.Call(func() error {
		return errors.New("error 2")
	})
	
	assert.Equal(t, StateOpen, breaker.State())
	
	time.Sleep(150 * time.Millisecond)
	
	err := breaker.Call(func() error {
		return nil
	})
	
	assert.NoError(t, err)
}

func TestBreakerRecovery(t *testing.T) {
	breaker := NewBreaker("test", 2, 100*time.Millisecond)
	breaker.halfOpenSuccesses = 3
	
	breaker.RecordFailure()
	breaker.RecordFailure()
	
	assert.Equal(t, StateOpen, breaker.State())
	
	time.Sleep(150 * time.Millisecond)
	
	assert.True(t, breaker.Allow())
	
	for i := 0; i < 3; i++ {
		breaker.RecordSuccess()
	}
	
	assert.Equal(t, StateClosed, breaker.State())
}

func TestBreakerStats(t *testing.T) {
	breaker := NewBreaker("test", 5, 1*time.Second)
	
	breaker.RecordSuccess()
	breaker.RecordFailure()
	
	stats := breaker.Stats()
	
	assert.Equal(t, "test", stats.Name)
	assert.Equal(t, StateClosed, stats.State)
	assert.Equal(t, int64(1), stats.Failures)
}

func TestBreakerReset(t *testing.T) {
	breaker := NewBreaker("test", 2, 1*time.Second)
	
	breaker.RecordFailure()
	breaker.RecordFailure()
	
	assert.Equal(t, StateOpen, breaker.State())
	
	breaker.Reset()
	
	assert.Equal(t, StateClosed, breaker.State())
	assert.Equal(t, int64(0), breaker.failures)
	assert.Equal(t, int64(0), breaker.successes)
}

func TestBreakerHalfOpenFailure(t *testing.T) {
	breaker := NewBreaker("test", 2, 50*time.Millisecond)
	
	breaker.RecordFailure()
	breaker.RecordFailure()
	assert.Equal(t, StateOpen, breaker.State())
	
	time.Sleep(100 * time.Millisecond)
	
	assert.True(t, breaker.Allow())
	assert.Equal(t, StateHalfOpen, breaker.State())
	
	breaker.RecordFailure()
	assert.Equal(t, StateOpen, breaker.State())
}

func TestBreakerConcurrent(t *testing.T) {
	breaker := NewBreaker("test", 50, 1*time.Second)
	
	done := make(chan bool)
	successCount := int64(0)
	failureCount := int64(0)
	
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				if j%2 == 0 {
					breaker.RecordSuccess()
					atomic.AddInt64(&successCount, 1)
				} else {
					breaker.RecordFailure()
					atomic.AddInt64(&failureCount, 1)
				}
			}
			done <- true
		}()
	}
	
	for i := 0; i < 10; i++ {
		<-done
	}
	
	total := atomic.LoadInt64(&successCount) + atomic.LoadInt64(&failureCount)
	assert.Equal(t, int64(1000), total)
}

func BenchmarkBreakerCall(b *testing.B) {
	breaker := NewBreaker("test", 1000, 10*time.Second)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_ = breaker.Call(func() error {
			return nil
		})
	}
}
