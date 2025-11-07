package worker

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"
)

type Job interface {
	Process(ctx context.Context) error
	GetID() string
}

type Pool struct {
	jobs       chan Job
	results    chan error
	workers    int
	ctx        context.Context
	cancel     context.CancelFunc
	processing int32
	
	totalProcessed int64
	totalErrors    int64
}

func NewPool(ctx context.Context, workers int, bufferSize int) *Pool {
	ctx, cancel := context.WithCancel(ctx)
	
	pool := &Pool{
		jobs:    make(chan Job, bufferSize),
		results: make(chan error, bufferSize),
		workers: workers,
		ctx:     ctx,
		cancel:  cancel,
	}
	
	for i := 0; i < workers; i++ {
		go pool.worker(i)
	}
	
	go pool.resultCollector()
	
	log.Printf("Worker pool inicializado: %d workers, buffer de %d", workers, bufferSize)
	
	return pool
}

func (p *Pool) worker(id int) {
	for {
		select {
		case <-p.ctx.Done():
			return
			
		case job, ok := <-p.jobs:
			if !ok {
				return
			}
			
			atomic.AddInt32(&p.processing, 1)
			
			err := job.Process(p.ctx)
			
			atomic.AddInt32(&p.processing, -1)
			atomic.AddInt64(&p.totalProcessed, 1)
			
			if err != nil {
				atomic.AddInt64(&p.totalErrors, 1)
			}
			
			select {
			case p.results <- err:
			case <-p.ctx.Done():
				return
			default:
			}
		}
	}
}

func (p *Pool) resultCollector() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-p.ctx.Done():
			return
			
		case <-ticker.C:
			processed := atomic.LoadInt64(&p.totalProcessed)
			errors := atomic.LoadInt64(&p.totalErrors)
			if processed > 0 {
				errorRate := float64(errors) / float64(processed) * 100
				log.Printf("Worker pool stats: %d processados, %d erros (%.2f%%), %d em processamento",
					processed, errors, errorRate, atomic.LoadInt32(&p.processing))
			}
			
		case err := <-p.results:
			if err != nil {
				errorCount := atomic.LoadInt64(&p.totalErrors)
				if errorCount%100 == 0 {
					log.Printf("Worker pool: %d erros acumulados", errorCount)
				}
			}
		}
	}
}

func (p *Pool) Submit(job Job) error {
	select {
	case p.jobs <- job:
		return nil
	case <-p.ctx.Done():
		return fmt.Errorf("pool fechado")
	default:
		return fmt.Errorf("buffer cheio")
	}
}

func (p *Pool) SubmitNonBlocking(job Job) bool {
	select {
	case p.jobs <- job:
		return true
	default:
		return false
	}
}

func (p *Pool) Close() {
	log.Println("Fechando worker pool...")
	close(p.jobs)
	
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-timeout:
			log.Printf("Timeout: %d jobs ainda processando", atomic.LoadInt32(&p.processing))
			p.cancel()
			return
			
		case <-ticker.C:
			if atomic.LoadInt32(&p.processing) == 0 {
				log.Println("Worker pool finalizado")
				p.cancel()
				return
			}
		}
	}
}

func (p *Pool) Stats() PoolStats {
	return PoolStats{
		Workers:        p.workers,
		QueueSize:      len(p.jobs),
		Processing:     int(atomic.LoadInt32(&p.processing)),
		Capacity:       cap(p.jobs),
		TotalProcessed: atomic.LoadInt64(&p.totalProcessed),
		TotalErrors:    atomic.LoadInt64(&p.totalErrors),
	}
}

type PoolStats struct {
	Workers        int
	QueueSize      int
	Processing     int
	Capacity       int
	TotalProcessed int64
	TotalErrors    int64
}

func (ps PoolStats) String() string {
	return fmt.Sprintf("Workers: %d, Queue: %d/%d, Processing: %d, Total: %d (erros: %d)",
		ps.Workers, ps.QueueSize, ps.Capacity, ps.Processing, ps.TotalProcessed, ps.TotalErrors)
}
