package waste

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/showwin/speedtest-go/speedtest"
)

type SpeedtestMode string

const (
	SpeedtestBest   SpeedtestMode = "best"
	SpeedtestRandom SpeedtestMode = "random"
	SpeedtestWorst  SpeedtestMode = "worst"
)

func Network(ctx context.Context, interval time.Duration, connectionCount int) {
	NetworkSpeedtest(ctx, interval, connectionCount, SpeedtestBest)
}

func NetworkSpeedtest(ctx context.Context, interval time.Duration, connectionCount int, mode SpeedtestMode) {
	if interval <= 0 {
		interval = 45 * time.Minute
	}
	const minInterval = 1 * time.Minute
	if interval < minInterval {
		fmt.Printf("[NETWORK] Interval terlalu kecil (%v). Gunakan minimum %v untuk stabilitas.\n", interval, minInterval)
		interval = minInterval
	}

	fmt.Printf("[NETWORK] Starting Network Waste Worker (mode=%s)...\n", mode)
	if connectionCount > 0 {
		fmt.Printf("[NETWORK] Target connections: %d (best-effort, library default may apply)\n", connectionCount)
	}

	client := speedtest.New()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	user, err := client.FetchUserInfo()
	if err != nil {
		fmt.Println("[NETWORK] Warning: Cannot fetch user info:", err)
	} else {
		fmt.Printf("[NETWORK] IP: %s | ISP: %s\n", user.IP, user.Isp)
	}

	const baseBackoff = 1 * time.Minute
	const maxBackoff = 15 * time.Minute
	failures := 0

	for {
		select {
		case <-ctx.Done():
			fmt.Println("[NETWORK] Stop signal received. Exiting network worker.")
			return
		default:
		}

		runCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)

		serverList, err := client.FetchServers()
		if err != nil {
			fmt.Println("[NETWORK] Error fetching servers:", err)
			cancel()
			failures++
			sleepWithBackoff(interval, failures, baseBackoff, maxBackoff)
			continue
		}

		server, err := pickSpeedtestServer(serverList, mode, rng)
		if err != nil {
			fmt.Println("[NETWORK] No servers found.")
			cancel()
			failures++
			sleepWithBackoff(interval, failures, baseBackoff, maxBackoff)
			continue
		}
		fmt.Printf("[NETWORK] Testing against: %s (%s)\n", server.Sponsor, server.Name)

		err = server.PingTest(nil)

		if err != nil {
			fmt.Println("[NETWORK] Ping failed:", err)
			failures++
		} else {
			dlCtx, dlCancel := context.WithTimeout(runCtx, 45*time.Second)
			err = server.DownloadTestContext(dlCtx)
			dlCancel()
			if err != nil {
				fmt.Println("[NETWORK] Download warning:", err)
				failures++
			}

			ulCtx, ulCancel := context.WithTimeout(runCtx, 45*time.Second)
			err = server.UploadTestContext(ulCtx)
			ulCancel()
			if err != nil {
				fmt.Println("[NETWORK] Upload warning:", err)
				failures++
			}

			fmt.Printf("[NETWORK] Result -> Ping: %d ms | DL: %.2f Mbps | UL: %.2f Mbps\n",
				server.Latency.Milliseconds(), server.DLSpeed, server.ULSpeed)
		}

		cancel()
		if failures == 0 {
			fmt.Printf("[NETWORK] Sleeping for %v...\n", interval)
			time.Sleep(interval)
		} else {
			sleepWithBackoff(interval, failures, baseBackoff, maxBackoff)
		}
	}
}

func pickSpeedtestServer(servers speedtest.Servers, mode SpeedtestMode, rng *rand.Rand) (*speedtest.Server, error) {
	if len(servers) == 0 {
		return nil, speedtest.ErrServerNotFound
	}

	switch mode {
	case SpeedtestRandom:
		available := servers.Available()
		pool := servers
		if available != nil && len(*available) > 0 {
			pool = *available
		}
		return pool[rng.Intn(len(pool))], nil
	case SpeedtestWorst:
		available := servers.Available()
		pool := servers
		if available != nil && len(*available) > 0 {
			pool = *available
		}
		if len(pool) == 0 {
			return nil, speedtest.ErrServerNotFound
		}
		start := len(pool) * 3 / 4
		if start >= len(pool) {
			start = len(pool) - 1
		}
		if start < 0 {
			start = 0
		}
		worstPool := pool[start:]
		return worstPool[rng.Intn(len(worstPool))], nil
	default:
		targets, err := servers.FindServer([]int{})
		if err != nil || len(targets) == 0 {
			return nil, speedtest.ErrServerNotFound
		}
		return targets[0], nil
	}
}

func sleepWithBackoff(interval time.Duration, failures int, base time.Duration, max time.Duration) {
	backoff := time.Duration(failures) * base
	if backoff > max {
		backoff = max
	}
	sleepFor := interval + backoff
	fmt.Printf("[NETWORK] Backoff %v (failures=%d). Sleeping...\n", sleepFor, failures)
	time.Sleep(sleepFor)
}
