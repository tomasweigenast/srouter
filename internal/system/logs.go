package system

import (
	"bufio"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type LogLine struct {
	Raw      string
	Category string
	Time     string
	Message  string
}

type LogFilter struct {
	Category string
	Search   string
}

type logSubscriber struct {
	ch     chan LogLine
	filter LogFilter
}

const historySize = 300

// LogBroadcaster tails a log file and fans out filtered lines to subscribers.
type LogBroadcaster struct {
	path        string
	mu          sync.Mutex
	subscribers map[int]*logSubscriber
	nextID      int
	stopCh      chan struct{}
	history     []LogLine // ring buffer of recent lines for new subscribers
}

func NewLogBroadcaster(path string) *LogBroadcaster {
	lb := &LogBroadcaster{
		path:        path,
		subscribers: map[int]*logSubscriber{},
		stopCh:      make(chan struct{}),
	}
	go lb.run()
	return lb
}

func (lb *LogBroadcaster) Subscribe(f LogFilter) (<-chan LogLine, func()) {
	lb.mu.Lock()
	id := lb.nextID
	lb.nextID++
	sub := &logSubscriber{ch: make(chan LogLine, historySize+64), filter: f}
	// Replay recent history matching the filter so the page fills immediately
	for _, line := range lb.history {
		if matchesFilter(line, f) {
			sub.ch <- line
		}
	}
	lb.subscribers[id] = sub
	lb.mu.Unlock()

	return sub.ch, func() {
		lb.mu.Lock()
		delete(lb.subscribers, id)
		close(sub.ch)
		lb.mu.Unlock()
	}
}

func (lb *LogBroadcaster) Stop() {
	close(lb.stopCh)
}

func (lb *LogBroadcaster) run() {
	f, err := os.Open(lb.path)
	if err != nil {
		return
	}
	defer f.Close()

	// Seek back to show recent history on first connect (~last 100 lines)
	const tailBytes = 32 * 1024
	if _, err := f.Seek(-tailBytes, io.SeekEnd); err != nil {
		f.Seek(0, io.SeekStart)
	}

	reader := bufio.NewReader(f)
	reader.ReadString('\n') // discard partial first line at seek boundary
	for {
		select {
		case <-lb.stopCh:
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		line = strings.TrimRight(line, "\n\r")
		if line == "" {
			continue
		}

		parsed := ParseLogLine(line)

		lb.mu.Lock()
		lb.history = append(lb.history, parsed)
		if len(lb.history) > historySize {
			lb.history = lb.history[len(lb.history)-historySize:]
		}
		for _, sub := range lb.subscribers {
			if matchesFilter(parsed, sub.filter) {
				select {
				case sub.ch <- parsed:
				default:
				}
			}
		}
		lb.mu.Unlock()
	}
}

func matchesFilter(l LogLine, f LogFilter) bool {
	if f.Category != "" && l.Category != f.Category {
		return false
	}
	if f.Search != "" && !strings.Contains(strings.ToLower(l.Raw), strings.ToLower(f.Search)) {
		return false
	}
	return true
}

func ParseLogLine(raw string) LogLine {
	l := LogLine{Raw: raw}

	// Alpine syslog format: "Jan  2 15:04:05 hostname service: message"
	parts := strings.SplitN(raw, " ", 5)
	if len(parts) >= 5 {
		l.Time = strings.Join(parts[:3], " ")
		l.Message = parts[4]
	} else {
		l.Message = raw
	}

	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "iptables") || strings.Contains(lower, "fw-input") || strings.Contains(lower, "mc-connect"):
		l.Category = "firewall"
	case strings.Contains(lower, "dnsmasq"):
		l.Category = "dhcp"
	case strings.Contains(lower, "pppd") || strings.Contains(lower, "pppoe"):
		l.Category = "pppoe"
	default:
		l.Category = "system"
	}

	return l
}
