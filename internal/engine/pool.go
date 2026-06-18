package engine

import (
	"context"
	"runtime"
	"sync"
)

type Job struct {
	Data any
}

type Result struct {
	Job  Job
	Err  error
	Info string
	Name string
}

type WorkerPool struct {
	workers int
	wg      sync.WaitGroup
	cancel  context.CancelFunc
}

func NewPool(n int) *WorkerPool {
	if n <= 0 {
		n = runtime.NumCPU()
	}
	return &WorkerPool{workers: n}
}

func (p *WorkerPool) Start(ctx context.Context, handler func(context.Context, any) Result) (chan<- Job, <-chan Result) {
	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	jobs := make(chan Job, p.workers*2)
	results := make(chan Result, p.workers*2)

	workerFn := func() {
		defer p.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case job, ok := <-jobs:
				if !ok {
					return
				}
				res := handler(ctx, job.Data)
				res.Job = job
				results <- res
			}
		}
	}

	p.wg.Add(p.workers)
	for i := 0; i < p.workers; i++ {
		go workerFn()
	}

	return jobs, results
}

func (p *WorkerPool) Wait() {
	p.wg.Wait()
	if p.cancel != nil {
		p.cancel()
	}
}
