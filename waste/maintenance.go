package waste

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	mathrand "math/rand"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
)

type MaintenanceConfig struct {
	ComputeMinInterval time.Duration
	ComputeMaxInterval time.Duration
	ComputeActiveMin   time.Duration
	ComputeActiveMax   time.Duration
	ComputeIdleMin     time.Duration
	ComputeIdleMax     time.Duration
	ComputeRatioMin    float64
	ComputeRatioMax    float64
	ComputeRatioCap    float64
	ComputeWorkersMin  int
	ComputeWorkersMax  int

	NetworkMinInterval time.Duration
	NetworkMaxInterval time.Duration
	NetworkMinRequests int
	NetworkMaxRequests int

	MemoryMinInterval time.Duration
	MemoryMaxInterval time.Duration
	MemoryMinMiB      int
	MemoryMaxMiB      int

	HeartbeatURL         string
	HeartbeatMinInterval time.Duration
	HeartbeatMaxInterval time.Duration
}

type MaintenanceRunner struct {
	wg sync.WaitGroup
}

func StartMaintenance(ctx context.Context, cfg MaintenanceConfig) *MaintenanceRunner {
	cfg = applyMaintenanceDefaults(cfg)
	runner := &MaintenanceRunner{}
	runner.wg.Add(1)
	go func() {
		defer runner.wg.Done()
		runComputeLoop(ctx, cfg)
	}()

	runner.wg.Add(1)
	go func() {
		defer runner.wg.Done()
		runNetworkLoop(ctx, cfg)
	}()

	if runtime.GOARCH == "arm64" {
		runner.wg.Add(1)
		go func() {
			defer runner.wg.Done()
			runMemoryLoop(ctx, cfg)
		}()
	}

	if cfg.HeartbeatURL != "" {
		runner.wg.Add(1)
		go func() {
			defer runner.wg.Done()
			runHeartbeatLoop(ctx, cfg)
		}()
	}

	return runner
}

func (r *MaintenanceRunner) Wait() {
	if r == nil {
		return
	}
	r.wg.Wait()
}

func applyMaintenanceDefaults(cfg MaintenanceConfig) MaintenanceConfig {
	if cfg.ComputeMinInterval <= 0 {
		cfg.ComputeMinInterval = 7 * time.Minute
	}
	if cfg.ComputeMaxInterval <= 0 {
		cfg.ComputeMaxInterval = 35 * time.Minute
	}
	if cfg.ComputeMaxInterval < cfg.ComputeMinInterval {
		cfg.ComputeMaxInterval = cfg.ComputeMinInterval
	}
	if cfg.ComputeActiveMin <= 0 {
		cfg.ComputeActiveMin = 60 * time.Second
	}
	if cfg.ComputeActiveMax <= 0 {
		cfg.ComputeActiveMax = 240 * time.Second
	}
	if cfg.ComputeActiveMax < cfg.ComputeActiveMin {
		cfg.ComputeActiveMax = cfg.ComputeActiveMin
	}
	if cfg.ComputeIdleMin <= 0 {
		cfg.ComputeIdleMin = 3 * time.Minute
	}
	if cfg.ComputeIdleMax <= 0 {
		cfg.ComputeIdleMax = 15 * time.Minute
	}
	if cfg.ComputeIdleMax < cfg.ComputeIdleMin {
		cfg.ComputeIdleMax = cfg.ComputeIdleMin
	}
	if cfg.ComputeRatioMin <= 0 {
		cfg.ComputeRatioMin = 0.25
	}
	if cfg.ComputeRatioMax <= 0 {
		cfg.ComputeRatioMax = 0.35
	}
	if cfg.ComputeRatioMax < cfg.ComputeRatioMin {
		cfg.ComputeRatioMax = cfg.ComputeRatioMin
	}
	if cfg.ComputeRatioCap <= 0 {
		cfg.ComputeRatioCap = 0.70
	}
	if cfg.ComputeWorkersMin <= 0 {
		cfg.ComputeWorkersMin = 4
	}
	if cfg.ComputeWorkersMax <= 0 {
		cfg.ComputeWorkersMax = 8
	}
	if cfg.ComputeWorkersMax < cfg.ComputeWorkersMin {
		cfg.ComputeWorkersMax = cfg.ComputeWorkersMin
	}
	if cfg.NetworkMinInterval <= 0 {
		cfg.NetworkMinInterval = 3 * time.Minute
	}
	if cfg.NetworkMaxInterval <= 0 {
		cfg.NetworkMaxInterval = 15 * time.Minute
	}
	if cfg.NetworkMaxInterval < cfg.NetworkMinInterval {
		cfg.NetworkMaxInterval = cfg.NetworkMinInterval
	}
	if cfg.NetworkMinRequests <= 0 {
		cfg.NetworkMinRequests = 2
	}
	if cfg.NetworkMaxRequests <= 0 {
		cfg.NetworkMaxRequests = 6
	}
	if cfg.NetworkMaxRequests < cfg.NetworkMinRequests {
		cfg.NetworkMaxRequests = cfg.NetworkMinRequests
	}
	if cfg.MemoryMinInterval <= 0 {
		cfg.MemoryMinInterval = 4 * time.Minute
	}
	if cfg.MemoryMaxInterval <= 0 {
		cfg.MemoryMaxInterval = 18 * time.Minute
	}
	if cfg.MemoryMaxInterval < cfg.MemoryMinInterval {
		cfg.MemoryMaxInterval = cfg.MemoryMinInterval
	}
	if cfg.MemoryMinMiB <= 0 {
		cfg.MemoryMinMiB = 300
	}
	if cfg.MemoryMaxMiB <= 0 {
		cfg.MemoryMaxMiB = 900
	}
	if cfg.MemoryMaxMiB < cfg.MemoryMinMiB {
		cfg.MemoryMaxMiB = cfg.MemoryMinMiB
	}
	if cfg.HeartbeatMinInterval <= 0 {
		cfg.HeartbeatMinInterval = 60 * time.Minute
	}
	if cfg.HeartbeatMaxInterval <= 0 {
		cfg.HeartbeatMaxInterval = 120 * time.Minute
	}
	if cfg.HeartbeatMaxInterval < cfg.HeartbeatMinInterval {
		cfg.HeartbeatMaxInterval = cfg.HeartbeatMinInterval
	}
	return cfg
}

func runComputeLoop(ctx context.Context, cfg MaintenanceConfig) {
	rng := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
	for {
		if !sleepWithContext(ctx, jitterDuration(rng, cfg.ComputeMinInterval, cfg.ComputeMaxInterval)) {
			return
		}

		active := jitterDuration(rng, cfg.ComputeActiveMin, cfg.ComputeActiveMax)
		idle := jitterDuration(rng, cfg.ComputeIdleMin, cfg.ComputeIdleMax)
		workers := randomInt(rng, cfg.ComputeWorkersMin, cfg.ComputeWorkersMax)
		target := randomRatio(rng, cfg.ComputeRatioMin, cfg.ComputeRatioMax)
		if target > cfg.ComputeRatioCap {
			target = cfg.ComputeRatioCap
		}

		cpuPercent := sampleCPUPercent(200 * time.Millisecond)
		start := time.Now()
		bytesProcessed := runComputeWorkload(ctx, active, workers, target, cfg.ComputeRatioCap)
		fmt.Printf("[MAINT][COMPUTE] Start=%s CPU=%.1f%% Workers=%d Target=%.0f%% Processed=%.2f MiB Duration=%s\n",
			start.Format(time.RFC3339),
			cpuPercent,
			workers,
			target*100,
			float64(bytesProcessed)/float64(MiB),
			active,
		)

		if !sleepWithContext(ctx, idle) {
			return
		}
	}
}

func runComputeWorkload(ctx context.Context, duration time.Duration, workers int, targetRatio float64, capRatio float64) uint64 {
	if workers <= 0 {
		workers = 1
	}
	interval := 200 * time.Millisecond
	ratioBits := &atomic.Uint64{}
	ratioBits.Store(math.Float64bits(targetRatio))

	var bytesProcessed atomic.Uint64
	var wg sync.WaitGroup
	workerCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			buf := make([]byte, 256*1024)
			if _, err := rand.Read(buf); err != nil {
				return
			}
			timer := time.NewTimer(0)
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			defer timer.Stop()

			for {
				select {
				case <-workerCtx.Done():
					return
				default:
				}

				ratio := math.Float64frombits(ratioBits.Load())
				if ratio < 0 {
					ratio = 0
				}
				if ratio > capRatio {
					ratio = capRatio
				}

				busy := time.Duration(float64(interval) * ratio)
				idle := interval - busy
				if idle < 0 {
					idle = 0
				}

				if busy > 0 {
					deadline := time.Now().Add(busy)
					for time.Now().Before(deadline) {
						sum := sha256.Sum256(buf)
						bytesProcessed.Add(uint64(len(buf)))
						buf[0] ^= sum[0]
					}
				}

				if idle > 0 {
					timer.Reset(idle)
					select {
					case <-workerCtx.Done():
						return
					case <-timer.C:
					}
				}
			}
		}()
	}

	smoothingCtx, smoothingCancel := context.WithCancel(workerCtx)
	defer smoothingCancel()
	go func() {
		rng := mathrand.New(mathrand.NewSource(time.Now().UnixNano() + 1))
		current := targetRatio
		stepInterval := 5 * time.Second
		timer := time.NewTimer(stepInterval)
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		defer timer.Stop()

		for {
			timer.Reset(stepInterval)
			select {
			case <-smoothingCtx.Done():
				return
			case <-timer.C:
			}

			target := randomRatio(rng, cfgClamp(cfgComputeRatioMin(targetRatio), 0.10, capRatio), cfgClamp(cfgComputeRatioMax(targetRatio), 0.10, capRatio))
			delta := target - current
			current += delta * 0.35
			if current > capRatio {
				current = capRatio
			}
			if current < 0 {
				current = 0
			}
			ratioBits.Store(math.Float64bits(current))
		}
	}()

	wg.Wait()
	return bytesProcessed.Load()
}

func cfgClamp(value float64, min float64, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func cfgComputeRatioMin(base float64) float64 {
	return base * 0.8
}

func cfgComputeRatioMax(base float64) float64 {
	return base * 1.2
}

func runNetworkLoop(ctx context.Context, cfg MaintenanceConfig) {
	rng := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
	client := &http.Client{Timeout: 15 * time.Second}
	endpoints := []string{
		"https://cloudflare.com/cdn-cgi/trace",
		"https://api.github.com/zen",
		"https://en.wikipedia.org/api/rest_v1/page/random/summary",
		"https://www.google.com/generate_204",
		"https://ident.me",
	}
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 13_6) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:119.0) Gecko/20100101 Firefox/119.0",
	}

	for {
		if !sleepWithContext(ctx, jitterDuration(rng, cfg.NetworkMinInterval, cfg.NetworkMaxInterval)) {
			return
		}

		reqCount := randomInt(rng, cfg.NetworkMinRequests, cfg.NetworkMaxRequests)
		if reqCount > len(endpoints) {
			reqCount = len(endpoints)
		}

		rng.Shuffle(len(endpoints), func(i, j int) {
			endpoints[i], endpoints[j] = endpoints[j], endpoints[i]
		})

		if rng.Intn(4) == 0 {
			dnsCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, err := net.DefaultResolver.LookupTXT(dnsCtx, "cloudflare.com")
			cancel()
			if err != nil {
				fmt.Printf("[MAINT][NET] DNS check error: %v\n", err)
			}
		}

		var totalBytes int64
		cpuPercent := sampleCPUPercent(200 * time.Millisecond)
		start := time.Now()
		for i := 0; i < reqCount; i++ {
			endpoint := endpoints[i]
			ua := userAgents[rng.Intn(len(userAgents))]
			bytesRead, err := fetchEndpoint(ctx, client, endpoint, ua)
			if err != nil {
				fmt.Printf("[MAINT][NET] %s error: %v\n", endpoint, err)
				continue
			}
			totalBytes += bytesRead
		}

		fmt.Printf("[MAINT][NET] Start=%s CPU=%.1f%% Requests=%d Bytes=%.2f MiB\n",
			start.Format(time.RFC3339),
			cpuPercent,
			reqCount,
			float64(totalBytes)/float64(MiB),
		)
	}
}

func fetchEndpoint(ctx context.Context, client *http.Client, endpoint string, userAgent string) (int64, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	bytesRead, err := io.Copy(io.Discard, io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return bytesRead, err
	}
	if resp.StatusCode >= 400 {
		return bytesRead, fmt.Errorf("status %d", resp.StatusCode)
	}
	return bytesRead, nil
}

func runMemoryLoop(ctx context.Context, cfg MaintenanceConfig) {
	rng := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
	for {
		if !sleepWithContext(ctx, jitterDuration(rng, cfg.MemoryMinInterval, cfg.MemoryMaxInterval)) {
			return
		}

		sizeMiB := randomInt(rng, cfg.MemoryMinMiB, cfg.MemoryMaxMiB)
		if sizeMiB <= 0 {
			continue
		}

		cpuPercent := sampleCPUPercent(200 * time.Millisecond)
		start := time.Now()
		checksum := memoryLifecycle(sizeMiB)
		fmt.Printf("[MAINT][MEM] Start=%s CPU=%.1f%% Size=%d MiB Checksum=%s\n",
			start.Format(time.RFC3339),
			cpuPercent,
			sizeMiB,
			checksum,
		)
	}
}

func memoryLifecycle(sizeMiB int) string {
	sizeBytes := sizeMiB * MiB
	buf := make([]byte, sizeBytes)
	for i := 0; i < len(buf); i += page {
		buf[i] = byte(i % 256)
	}
	var sum byte
	for i := 0; i < len(buf); i += page {
		sum ^= buf[i]
	}
	result := hex.EncodeToString([]byte{sum})
	buf = nil
	runtime.GC()
	return result
}

func runHeartbeatLoop(ctx context.Context, cfg MaintenanceConfig) {
	rng := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
	client := &http.Client{Timeout: 10 * time.Second}
	for {
		if !sleepWithContext(ctx, jitterDuration(rng, cfg.HeartbeatMinInterval, cfg.HeartbeatMaxInterval)) {
			return
		}

		payload := fmt.Sprintf(`{"status":"ok","time":"%s"}`, time.Now().Format(time.RFC3339))
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.HeartbeatURL, strings.NewReader(payload))
		if err != nil {
			fmt.Printf("[MAINT][HEARTBEAT] request error: %v\n", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("[MAINT][HEARTBEAT] send error: %v\n", err)
			continue
		}
		_ = resp.Body.Close()
		fmt.Printf("[MAINT][HEARTBEAT] Sent status %d\n", resp.StatusCode)
	}
}

func jitterDuration(rng *mathrand.Rand, min time.Duration, max time.Duration) time.Duration {
	if max <= min {
		return min
	}
	delta := max - min
	return min + time.Duration(rng.Int63n(int64(delta)+1))
}

func randomInt(rng *mathrand.Rand, min int, max int) int {
	if max <= min {
		return min
	}
	return min + rng.Intn(max-min+1)
}

func sampleCPUPercent(sample time.Duration) float64 {
	percents, err := cpu.Percent(sample, false)
	if err != nil || len(percents) == 0 {
		return 0
	}
	return percents[0]
}
