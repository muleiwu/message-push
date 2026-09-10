package scheduler

import (
	"context"
	"sync"
	"time"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/service"
)

// SMSPollScanner separates cloud polling from local event dispatch so a slow
// upstream account cannot hold up processing of already persisted receipts.
type SMSPollScanner struct {
	service *service.SMSEventService
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewSMSPollScanner() *SMSPollScanner {
	return &SMSPollScanner{service: service.NewSMSEventService()}
}

func (s *SMSPollScanner) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	for _, operation := range []struct {
		interval time.Duration
		run      func(context.Context) error
	}{{time.Second, s.service.ProcessPending}, {5 * time.Second, s.service.PollDue}} {
		s.wg.Add(1)
		go func(interval time.Duration, run func(context.Context) error) {
			defer s.wg.Done()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				if err := run(ctx); err != nil && ctx.Err() == nil {
					helper.GetLogger().Warn("短信状态处理：" + err.Error())
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}(operation.interval, operation.run)
	}
}

func (s *SMSPollScanner) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}
