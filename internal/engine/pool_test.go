package engine

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewPool_Default(t *testing.T) {
	p := NewPool(0)
	if p.workers <= 0 { t.Fatal("worker count should be positive") }
}

func TestNewPool_Specific(t *testing.T) {
	if NewPool(3).workers != 3 { t.Fatal("expected 3") }
}

func TestNewPool_Negative(t *testing.T) {
	if NewPool(-5).workers <= 0 { t.Fatal("negative should fallback") }
}

func TestPool_ProcessAll(t *testing.T) {
	p := NewPool(4)
	ctx := context.Background()
	var c atomic.Int32
	jobs, results := p.Start(ctx, func(_ context.Context, _ any) Result {
		c.Add(1); return Result{Info: "ok"}
	})
	n := 100
	go func() {
		for i := 0; i < n; i++ { jobs <- Job{Data: i} }
		close(jobs)
	}()
	got := 0
	for range results { got++ }
	p.Wait()
	if got != n { t.Fatalf("got %d results, want %d", got, n) }
	if int(c.Load()) != n { t.Fatalf("handler called %d times", c.Load()) }
}

func TestPool_EmptyJobs(t *testing.T) {
	p := NewPool(2)
	jobs, results := p.Start(context.Background(), func(_ context.Context, _ any) Result { return Result{Info:"ok"} })
	close(jobs)
	n := 0
	for range results { n++ }
	p.Wait()
	if n != 0 { t.Fatal("expected 0 results") }
}

func TestPool_SingleJob(t *testing.T) {
	p := NewPool(1)
	jobs, results := p.Start(context.Background(), func(_ context.Context, d any) Result {
		return Result{Info: "ok", Name: "n"}
	})
	go func() { jobs <- Job{Data: 21}; close(jobs) }()
	r := <-results
	if r.Info != "ok" || r.Name != "n" || r.Job.Data.(int) != 21 { t.Fatalf("got %+v, want Info=ok Name=n Data=21", r) }
	p.Wait()
}

func TestPool_CancelBeforeStart(t *testing.T) {
	p := NewPool(4)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	jobs, results := p.Start(ctx, func(_ context.Context, _ any) Result { return Result{Info:"run"} })
	close(jobs)
	n := 0
	for range results { n++ }
	p.Wait()
	if n != 0 { t.Fatal("expected 0 results after cancel") }
}

func TestPool_CancelMidway(t *testing.T) {
	p := NewPool(2)
	ctx, cancel := context.WithCancel(context.Background())
	var cnt atomic.Int32
	jobs, results := p.Start(ctx, func(c context.Context, _ any) Result {
		cnt.Add(1)
		select { case <-c.Done(): case <-time.After(100*time.Millisecond): }
		return Result{Info:"done"}
	})
	go func() {
		for i := 0; i < 100; i++ { jobs <- Job{Data:i} }
		close(jobs)
	}()
	time.Sleep(5 * time.Millisecond)
	cancel()
	for range results {}
	p.Wait()
	_ = cnt.Load() // any count ok, must not hang
}

func TestPool_HandlerError(t *testing.T) {
	p := NewPool(2)
	jobs, results := p.Start(context.Background(), func(_ context.Context, _ any) Result {
		return Result{Err: errors.New("fail"), Info: "error"}
	})
	go func() { jobs <- Job{Data:1}; close(jobs) }()
	r := <-results
	if r.Err == nil || r.Info != "error" { t.Fatal("result not propagated") }
	p.Wait()
}

func TestPool_ManyJobs(t *testing.T) {
	p := NewPool(8)
	var c atomic.Int32
	jobs, results := p.Start(context.Background(), func(_ context.Context, _ any) Result {
		c.Add(1); return Result{Info:"ok"}
	})
	n := 2000
	go func() {
		for i := 0; i < n; i++ { jobs <- Job{Data:i} }
		close(jobs)
	}()
	for range results {}
	p.Wait()
	if int(c.Load()) != n { t.Fatalf("called %d times", c.Load()) }
}

func TestPool_AnyDataType(t *testing.T) {
	p := NewPool(2)
	jobs, results := p.Start(context.Background(), func(_ context.Context, d any) Result {
		switch d.(type) {
		case int:    return Result{Info:"int"}
		case string: return Result{Info:"string"}
		default:     return Result{Info:"other"}
	}
	})
	go func() {
		jobs <- Job{Data:42}; jobs <- Job{Data:"hello"}; jobs <- Job{Data:3.14}
		close(jobs)
	}()
	m := map[string]int{}
	for r := range results { m[r.Info]++ }
	p.Wait()
	if m["int"]!=1 || m["string"]!=1 || m["other"]!=1 { t.Fatalf("%v", m) }
}

func TestPool_DoubleWait(t *testing.T) {
	p := NewPool(2)
	jobs, results := p.Start(context.Background(), func(_ context.Context, _ any) Result { return Result{Info:"ok"} })
	close(jobs)
	for range results {}
	p.Wait()
	p.Wait() // must not panic
}
