package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type TestJob struct {
	id        string
	shouldErr bool
	delay     time.Duration
	processed int32
}

func (j *TestJob) GetID() string {
	return j.id
}

func (j *TestJob) Process(ctx context.Context) error {
	if j.delay > 0 {
		time.Sleep(j.delay)
	}
	
	atomic.AddInt32(&j.processed, 1)
	
	if j.shouldErr {
		return errors.New("test error")
	}
	
	return nil
}

func TestNewPool(t *testing.T) {
	ctx := context.Background()
	pool := NewPool(ctx, 5, 10)
	
	assert.NotNil(t, pool)
	assert.Equal(t, 5, pool.workers)
	assert.Equal(t, 10, cap(pool.jobs))
	
	pool.Close()
}

func TestPoolSubmit(t *testing.T) {
	ctx := context.Background()
	pool := NewPool(ctx, 2, 10)
	defer pool.Close()
	
	job := &TestJob{id: "test1"}
	
	err := pool.Submit(job)
	assert.NoError(t, err)
	
	time.Sleep(100 * time.Millisecond)
	
	assert.Equal(t, int32(1), atomic.LoadInt32(&job.processed))
}

func TestPoolSubmitMultiple(t *testing.T) {
	ctx := context.Background()
	pool := NewPool(ctx, 5, 100)
	defer pool.Close()
	
	jobCount := 50
	jobs := make([]*TestJob, jobCount)
	
	for i := 0; i < jobCount; i++ {
		jobs[i] = &TestJob{
			id:    string(rune(i)),
			delay: 10 * time.Millisecond,
		}
		err := pool.Submit(jobs[i])
		assert.NoError(t, err)
	}
	
	time.Sleep(2 * time.Second)
	
	for i, job := range jobs {
		assert.Equal(t, int32(1), atomic.LoadInt32(&job.processed),
			"Job %d não foi processado", i)
	}
	
	stats := pool.Stats()
	assert.Equal(t, int64(jobCount), stats.TotalProcessed)
}

func TestPoolBufferFull(t *testing.T) {
	ctx := context.Background()
	pool := NewPool(ctx, 1, 2)
	defer pool.Close()
	
	job1 := &TestJob{id: "job1", delay: 1 * time.Second}
	job2 := &TestJob{id: "job2", delay: 1 * time.Second}
	job3 := &TestJob{id: "job3", delay: 1 * time.Second}
	
	err := pool.Submit(job1)
	assert.NoError(t, err)
	
	time.Sleep(10 * time.Millisecond)
	
	err = pool.Submit(job2)
	assert.NoError(t, err)
	
	err = pool.Submit(job3)
	assert.NoError(t, err)
	
	err = pool.Submit(&TestJob{id: "job4"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "buffer cheio")
	
	time.Sleep(3 * time.Second)
	
	stats := pool.Stats()
	assert.Equal(t, int64(3), stats.TotalProcessed)
}

func TestPoolWithErrors(t *testing.T) {
	ctx := context.Background()
	pool := NewPool(ctx, 2, 10)
	defer pool.Close()
	
	successJob := &TestJob{id: "success", shouldErr: false}
	errorJob := &TestJob{id: "error", shouldErr: true}
	
	err := pool.Submit(successJob)
	assert.NoError(t, err)
	
	err = pool.Submit(errorJob)
	assert.NoError(t, err)
	
	time.Sleep(200 * time.Millisecond)
	
	stats := pool.Stats()
	assert.Equal(t, int64(2), stats.TotalProcessed)
	assert.Equal(t, int64(1), stats.TotalErrors)
}

func TestPoolClose(t *testing.T) {
	ctx := context.Background()
	pool := NewPool(ctx, 2, 10)
	
	job := &TestJob{id: "test", delay: 100 * time.Millisecond}
	err := pool.Submit(job)
	assert.NoError(t, err)
	
	time.Sleep(200 * time.Millisecond)
	
	pool.Close()
	
	assert.Equal(t, int32(1), atomic.LoadInt32(&job.processed))
}

func TestPoolStats(t *testing.T) {
	ctx := context.Background()
	pool := NewPool(ctx, 3, 20)
	defer pool.Close()
	
	stats := pool.Stats()
	assert.Equal(t, 3, stats.Workers)
	assert.Equal(t, 20, stats.Capacity)
	assert.Equal(t, int64(0), stats.TotalProcessed)
	assert.Equal(t, int64(0), stats.TotalErrors)
	
	for i := 0; i < 10; i++ {
		job := &TestJob{
			id:        string(rune(i)),
			shouldErr: i%3 == 0,
		}
		_ = pool.Submit(job)
	}
	
	time.Sleep(500 * time.Millisecond)
	
	stats = pool.Stats()
	assert.Equal(t, int64(10), stats.TotalProcessed)
	assert.Greater(t, stats.TotalErrors, int64(0))
}

func BenchmarkPoolSubmit(b *testing.B) {
	ctx := context.Background()
	pool := NewPool(ctx, 10, 1000)
	defer pool.Close()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		job := &TestJob{id: string(rune(i))}
		_ = pool.Submit(job)
	}
	
	pool.Close()
}
