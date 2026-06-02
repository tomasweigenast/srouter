package system

import (
	"bufio"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tomasweigenast/srouter/internal/logging"
)

var bwLogger = logging.GetLogger("bandwidth")

type BandwidthSample struct {
	Timestamp int64
	Iface     string
	RxBps     float64
	TxBps     float64
}

type bwSubscriber struct {
	ch     chan BandwidthSample
	filter string // interface name filter, "" = all
}

// Broadcaster samples /proc/net/dev on a fixed interval and fans out to subscribers.
type Broadcaster struct {
	mu          sync.Mutex
	subscribers map[int]*bwSubscriber
	nextID      int
	stopCh      chan struct{}
	interval    time.Duration
}

func NewBroadcaster(interval time.Duration) *Broadcaster {
	b := &Broadcaster{
		subscribers: map[int]*bwSubscriber{},
		stopCh:      make(chan struct{}),
		interval:    interval,
	}
	go b.run()
	return b
}

func (b *Broadcaster) Subscribe() (<-chan BandwidthSample, func()) {
	b.mu.Lock()
	id := b.nextID
	b.nextID++
	sub := &bwSubscriber{ch: make(chan BandwidthSample, 4)}
	b.subscribers[id] = sub
	b.mu.Unlock()

	return sub.ch, func() {
		b.mu.Lock()
		delete(b.subscribers, id)
		close(sub.ch)
		b.mu.Unlock()
	}
}

func (b *Broadcaster) Stop() {
	close(b.stopCh)
}

func (b *Broadcaster) run() {
	prev := readNetDev()
	prevTime := time.Now()

	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case t := <-ticker.C:
			curr := readNetDev()
			elapsed := t.Sub(prevTime).Seconds()
			if elapsed <= 0 {
				elapsed = 1
			}

			var samples []BandwidthSample
			for iface, counts := range curr {
				if p, ok := prev[iface]; ok {
					samples = append(samples, BandwidthSample{
						Timestamp: t.UnixMilli(),
						Iface:     iface,
						RxBps:     float64(counts[0]-p[0]) / elapsed,
						TxBps:     float64(counts[1]-p[1]) / elapsed,
					})
				}
			}

			b.mu.Lock()
			for _, sub := range b.subscribers {
				for _, s := range samples {
					select {
					case sub.ch <- s:
					default:
					}
				}
			}
			b.mu.Unlock()

			prev = curr
			prevTime = t
		}
	}
}

func readNetDev() map[string][2]uint64 {
	result := map[string][2]uint64{}
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		bwLogger.Error("read /proc/net/dev", "err", err)
		return result
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Scan() // header 1
	scanner.Scan() // header 2
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}
		name := strings.TrimSuffix(fields[0], ":")
		var rx, tx uint64
		for i, v := range []string{fields[1], fields[9]} {
			var n uint64
			for _, c := range v {
				if c >= '0' && c <= '9' {
					n = n*10 + uint64(c-'0')
				}
			}
			if i == 0 {
				rx = n
			} else {
				tx = n
			}
		}
		result[name] = [2]uint64{rx, tx}
	}
	return result
}
