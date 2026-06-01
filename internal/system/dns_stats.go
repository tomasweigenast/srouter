package system

import (
	"sort"
	"strings"
	"sync"
)

type DNSStats struct {
	TotalQueries int64
	CacheHits    int64
	Forwarded    int64
	CacheHitRate float64
	ByUpstream   map[string]int64
	TopDomains   []DomainStat
}

type DomainStat struct {
	Domain string
	Count  int64
}

// DNSStatsCollector subscribes to the log stream and aggregates dnsmasq stats.
type DNSStatsCollector struct {
	mu         sync.RWMutex
	queries    int64
	cacheHits  int64
	forwarded  int64
	byUpstream map[string]int64
	byDomain   map[string]int64
	stopCh     chan struct{}
}

func NewDNSStatsCollector(logs LogStream) *DNSStatsCollector {
	c := &DNSStatsCollector{
		byUpstream: map[string]int64{},
		byDomain:   map[string]int64{},
		stopCh:     make(chan struct{}),
	}
	go c.run(logs)
	return c
}

func (c *DNSStatsCollector) run(logs LogStream) {
	ch, unsub := logs.Subscribe(LogFilter{Category: "dhcp"})
	defer unsub()
	for {
		select {
		case <-c.stopCh:
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			c.parseLine(line.Message)
		}
	}
}

func (c *DNSStatsCollector) parseLine(msg string) {
	lower := strings.ToLower(msg)
	c.mu.Lock()
	defer c.mu.Unlock()

	switch {
	case strings.Contains(lower, "query["):
		// "query[A] google.com from 192.168.0.x"
		c.queries++
		if domain := extractField(msg, 1); domain != "" {
			c.byDomain[domain]++
		}
	case strings.Contains(lower, "cached "):
		// "cached google.com is 1.2.3.4"
		c.cacheHits++
		c.queries++
		if domain := extractWordAfter(msg, "cached"); domain != "" {
			c.byDomain[domain]++
		}
	case strings.Contains(lower, "forwarded "):
		// "forwarded google.com to 1.1.1.1"
		c.forwarded++
		if upstream := extractWordAfter(msg, "to"); upstream != "" {
			c.byUpstream[upstream]++
		}
	}
}

func (c *DNSStatsCollector) Stats() DNSStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	upstreams := make(map[string]int64, len(c.byUpstream))
	for k, v := range c.byUpstream {
		upstreams[k] = v
	}

	domains := make([]DomainStat, 0, len(c.byDomain))
	for d, n := range c.byDomain {
		domains = append(domains, DomainStat{Domain: d, Count: n})
	}
	sort.Slice(domains, func(i, j int) bool { return domains[i].Count > domains[j].Count })
	if len(domains) > 10 {
		domains = domains[:10]
	}

	var hitRate float64
	if c.queries > 0 {
		hitRate = float64(c.cacheHits) / float64(c.queries) * 100
	}

	return DNSStats{
		TotalQueries: c.queries,
		CacheHits:    c.cacheHits,
		Forwarded:    c.forwarded,
		CacheHitRate: hitRate,
		ByUpstream:   upstreams,
		TopDomains:   domains,
	}
}

func (c *DNSStatsCollector) Stop() {
	close(c.stopCh)
}

// extractField returns the nth space-delimited word (0-based) from s.
func extractField(s string, n int) string {
	fields := strings.Fields(s)
	if n < len(fields) {
		return strings.TrimSuffix(fields[n], ".")
	}
	return ""
}

// extractWordAfter returns the word immediately following keyword in s.
func extractWordAfter(s, keyword string) string {
	lower := strings.ToLower(s)
	idx := strings.Index(lower, strings.ToLower(keyword))
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(s[idx+len(keyword):])
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
