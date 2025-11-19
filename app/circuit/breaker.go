package circuit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// State 熔断器状态
type State int

const (
	StateClosed   State = iota // 关闭（正常）
	StateOpen                  // 打开（熔断）
	StateHalfOpen              // 半开（探测）
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// Breaker 熔断器接口
type Breaker interface {
	Call(ctx context.Context, fn func() error) error
	State() State
	Reset()
}

// Counts 统计计数
type Counts struct {
	Requests             uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}

// SlidingWindowBreaker 滑动窗口熔断器
type SlidingWindowBreaker struct {
	name        string
	maxRequests uint32        // 半开状态下允许的最大请求数
	interval    time.Duration // 统计窗口时间
	timeout     time.Duration // 熔断超时时间（打开->半开）
	threshold   float64       // 失败率阈值
	minRequests uint32        // 最小请求数（达到才触发熔断）
	mu          sync.Mutex
	state       State
	generation  uint64
	counts      *Counts
	expiry      time.Time
}

// NewSlidingWindowBreaker 创建熔断器
func NewSlidingWindowBreaker(name string) *SlidingWindowBreaker {
	return &SlidingWindowBreaker{
		name:        name,
		maxRequests: 3,
		interval:    time.Minute,
		timeout:     time.Minute,
		threshold:   0.5,
		minRequests: 10,
		state:       StateClosed,
		counts:      &Counts{},
	}
}

// WithMaxRequests 设置半开状态最大请求数
func (b *SlidingWindowBreaker) WithMaxRequests(n uint32) *SlidingWindowBreaker {
	b.maxRequests = n
	return b
}

// WithInterval 设置统计窗口时间
func (b *SlidingWindowBreaker) WithInterval(d time.Duration) *SlidingWindowBreaker {
	b.interval = d
	return b
}

// WithTimeout 设置熔断超时时间
func (b *SlidingWindowBreaker) WithTimeout(d time.Duration) *SlidingWindowBreaker {
	b.timeout = d
	return b
}

// WithThreshold 设置失败率阈值
func (b *SlidingWindowBreaker) WithThreshold(t float64) *SlidingWindowBreaker {
	b.threshold = t
	return b
}

// Call 执行函数
func (b *SlidingWindowBreaker) Call(ctx context.Context, fn func() error) error {
	generation, err := b.beforeRequest()
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			b.afterRequest(generation, false)
			panic(r)
		}
	}()

	err = fn()
	b.afterRequest(generation, err == nil)

	return err
}

// State 获取当前状态
func (b *SlidingWindowBreaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	state, _ := b.currentState(now)

	return state
}

// Reset 重置熔断器
func (b *SlidingWindowBreaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state = StateClosed
	b.generation++
	b.counts = &Counts{}
	b.expiry = time.Time{}
}

// beforeRequest 请求前检查
func (b *SlidingWindowBreaker) beforeRequest() (uint64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	state, generation := b.currentState(now)

	if state == StateOpen {
		return generation, fmt.Errorf("circuit breaker %s is open", b.name)
	}

	if state == StateHalfOpen && b.counts.Requests >= b.maxRequests {
		return generation, fmt.Errorf("circuit breaker %s is half-open, too many requests", b.name)
	}

	b.counts.Requests++

	return generation, nil
}

// afterRequest 请求后处理
func (b *SlidingWindowBreaker) afterRequest(generation uint64, success bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	state, currentGen := b.currentState(now)

	if generation != currentGen {
		return
	}

	if success {
		b.onSuccess(state)
	} else {
		b.onFailure(state)
	}
}

// onSuccess 成功处理
func (b *SlidingWindowBreaker) onSuccess(state State) {
	b.counts.TotalSuccesses++
	b.counts.ConsecutiveSuccesses++
	b.counts.ConsecutiveFailures = 0

	if state == StateHalfOpen && b.counts.ConsecutiveSuccesses >= b.maxRequests {
		b.setState(StateClosed, time.Now())
	}
}

// onFailure 失败处理
func (b *SlidingWindowBreaker) onFailure(state State) {
	b.counts.TotalFailures++
	b.counts.ConsecutiveFailures++
	b.counts.ConsecutiveSuccesses = 0

	if state == StateHalfOpen {
		b.setState(StateOpen, time.Now())
		return
	}

	if b.counts.Requests >= b.minRequests {
		failureRate := float64(b.counts.TotalFailures) / float64(b.counts.Requests)

		if failureRate >= b.threshold {
			b.setState(StateOpen, time.Now())
		}
	}
}

// currentState 获取当前状态
func (b *SlidingWindowBreaker) currentState(now time.Time) (State, uint64) {
	switch b.state {
	case StateClosed:
		if !b.expiry.IsZero() && b.expiry.Before(now) {
			b.generation++
			b.counts = &Counts{}
			b.expiry = now.Add(b.interval)
		}

	case StateOpen:
		if b.expiry.Before(now) {
			b.setState(StateHalfOpen, now)
		}
	}

	return b.state, b.generation
}

// setState 设置状态
func (b *SlidingWindowBreaker) setState(state State, now time.Time) {
	if b.state == state {
		return
	}

	prev := b.state
	b.state = state
	b.generation++
	b.counts = &Counts{}

	var expiry time.Time
	switch state {
	case StateClosed:
		expiry = now.Add(b.interval)
	case StateOpen:
		expiry = now.Add(b.timeout)
	}
	b.expiry = expiry

	fmt.Printf("circuit breaker %s state changed: %s -> %s\n", b.name, prev, state)
}
