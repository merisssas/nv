package main

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/merisssas/nv/waste"
)

type cpuAutoController struct {
	mu     sync.Mutex
	burner *waste.Burner
	cfg    waste.Config
}

func (c *cpuAutoController) run(ctx context.Context, targetPercent float64, interval time.Duration, hysteresis float64) {
	if interval <= 0 {
		interval = 2 * time.Second
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		sample := 400 * time.Millisecond
		if interval < sample {
			sample = interval / 2
		}
		if sample <= 0 {
			sample = 200 * time.Millisecond
		}

		for {
			select {
			case <-ctx.Done():
				c.Stop()
				return
			case <-ticker.C:
			}

			usage, err := waste.CPUUsage(sample)
			if err != nil {
				continue
			}
			usagePercent := usage * 100
			upper := targetPercent + hysteresis
			lower := targetPercent - hysteresis

			c.mu.Lock()
			running := c.burner != nil
			c.mu.Unlock()

			switch {
			case running && usagePercent > upper:
				fmt.Printf("[AUTO][CPU] Usage %.1f%% > %.1f%%, pausing CPU waste.\n", usagePercent, upper)
				c.Stop()
			case !running && usagePercent < lower:
				fmt.Printf("[AUTO][CPU] Usage %.1f%% < %.1f%%, starting CPU waste.\n", usagePercent, lower)
				c.start()
			}
		}
	}()
}

func (c *cpuAutoController) start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.burner != nil {
		return
	}
	burner, err := waste.StartCPU(c.cfg)
	if err != nil {
		fmt.Printf("[AUTO][CPU] Error starting CPU waste: %v\n", err)
		return
	}
	c.burner = burner
}

func (c *cpuAutoController) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.burner != nil {
		c.burner.Stop()
		c.burner = nil
	}
}

type memoryAutoController struct {
	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
}

func (m *memoryAutoController) run(ctx context.Context, targetPercent float64, interval time.Duration, hysteresis float64) {
	if interval <= 0 {
		interval = 2 * time.Second
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				m.Stop()
				return
			case <-ticker.C:
			}

			stats, err := waste.MemoryUsage()
			if err != nil {
				continue
			}
			if stats.TotalBytes <= 0 {
				continue
			}
			usagePercent := float64(stats.UsedBytes) / float64(stats.TotalBytes) * 100
			upper := targetPercent + hysteresis
			lower := targetPercent - hysteresis

			m.mu.Lock()
			running := m.running
			m.mu.Unlock()

			switch {
			case running && usagePercent > upper:
				fmt.Printf("[AUTO][MEM] Usage %.1f%% > %.1f%%, releasing memory waste.\n", usagePercent, upper)
				m.Stop()
			case !running && usagePercent < lower:
				desired := int64(math.Round(float64(stats.TotalBytes) * targetPercent / 100))
				needed := desired - stats.UsedBytes
				if needed <= 0 {
					continue
				}
				fmt.Printf("[AUTO][MEM] Usage %.1f%% < %.1f%%, allocating ~%.2f GiB.\n",
					usagePercent,
					lower,
					float64(needed)/float64(waste.GiB),
				)
				m.start(ctx, needed)
			}
		}
	}()
}

func (m *memoryAutoController) start(ctx context.Context, targetBytes int64) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	memCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.running = true
	m.mu.Unlock()

	go func() {
		if err := waste.MemoryBytes(memCtx, targetBytes); err != nil {
			fmt.Printf("[AUTO][MEM] Error: %v\n", err)
		}
		m.mu.Lock()
		if m.cancel == cancel {
			m.cancel = nil
		}
		m.running = false
		m.mu.Unlock()
	}()
}

func (m *memoryAutoController) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.running = false
}
