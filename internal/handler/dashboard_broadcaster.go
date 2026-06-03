package handler

import (
	"sync"
	"time"
)

// DashboardSnapshot is a point-in-time capture of all dashboard data.
type DashboardSnapshot struct {
	Data    dashboardData
	Devices []Device
}

// DashboardBroadcaster runs a single background goroutine that calls a gather
// function on a fixed interval and fans the result out to all SSE subscribers.
// This ensures gatherData executes once per tick regardless of how many clients
// are connected, so rapid page refreshes cannot accumulate goroutines.
type DashboardBroadcaster struct {
	mu          sync.Mutex
	subscribers map[int]chan DashboardSnapshot
	nextID      int
	stopCh      chan struct{}

	gather   func() (dashboardData, []Device)
	interval time.Duration
}

func newDashboardBroadcaster(interval time.Duration, gather func() (dashboardData, []Device)) *DashboardBroadcaster {
	b := &DashboardBroadcaster{
		subscribers: map[int]chan DashboardSnapshot{},
		stopCh:      make(chan struct{}),
		gather:      gather,
		interval:    interval,
	}
	go b.run()
	return b
}

// Subscribe returns a channel that receives snapshots and a cleanup function.
// The caller must invoke the cleanup function when done (e.g. via defer).
func (b *DashboardBroadcaster) Subscribe() (<-chan DashboardSnapshot, func()) {
	b.mu.Lock()
	id := b.nextID
	b.nextID++
	ch := make(chan DashboardSnapshot, 2)
	b.subscribers[id] = ch
	b.mu.Unlock()

	return ch, func() {
		b.mu.Lock()
		delete(b.subscribers, id)
		close(ch)
		b.mu.Unlock()
	}
}

func (b *DashboardBroadcaster) Stop() {
	close(b.stopCh)
}

func (b *DashboardBroadcaster) run() {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			data, devices := b.gather()
			snap := DashboardSnapshot{Data: data, Devices: devices}

			b.mu.Lock()
			for _, ch := range b.subscribers {
				select {
				case ch <- snap:
				default:
				}
			}
			b.mu.Unlock()
		}
	}
}
