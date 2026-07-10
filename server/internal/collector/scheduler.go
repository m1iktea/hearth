package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// snapshotSink 是 Scheduler 对存储的最小依赖（由 store.SnapshotStore 满足）。
type snapshotSink interface {
	SetOK(source string, data any, at time.Time)
	SetError(source string, err error, at time.Time)
}

// Scheduler 定时并发触发所有 collector，将结果写入快照存储。
type Scheduler struct {
	collectors []Collector
	sink       snapshotSink
	interval   time.Duration
	logger     *slog.Logger
}

func NewScheduler(collectors []Collector, sink snapshotSink, interval time.Duration, logger *slog.Logger) *Scheduler {
	return &Scheduler{collectors: collectors, sink: sink, interval: interval, logger: logger}
}

// Run 先立即采集一次，然后按 interval 循环，直到 ctx 取消。
func (s *Scheduler) Run(ctx context.Context) {
	s.CollectOnce(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.CollectOnce(ctx)
		}
	}
}

// CollectOnce 并发采集所有源；单源超时上限为 interval，失败只影响该源。
func (s *Scheduler) CollectOnce(ctx context.Context) {
	var wg sync.WaitGroup
	for _, c := range s.collectors {
		wg.Add(1)
		go func(c Collector) {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, s.interval)
			defer cancel()
			data, err := c.Collect(cctx)
			now := time.Now()
			if err != nil {
				s.logger.Warn("collect failed", "source", c.Name(), "error", err)
				s.sink.SetError(c.Name(), err, now)
				return
			}
			s.sink.SetOK(c.Name(), data, now)
		}(c)
	}
	wg.Wait()
}
