package waste

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

var defaultDownloadTargets = []string{
	"https://speed.cloudflare.com/__down?bytes=200000",
	"https://speed.cloudflare.com/__down?bytes=500000",
	"https://www.cloudflare.com/cdn-cgi/trace",
}

func NetworkDownload(ctx context.Context, minInterval, maxInterval time.Duration, targets []string) {
	if minInterval <= 0 {
		minInterval = 5 * time.Minute
	}
	if maxInterval <= 0 {
		maxInterval = 10 * time.Minute
	}
	if maxInterval < minInterval {
		maxInterval = minInterval
	}
	if len(targets) == 0 {
		targets = defaultDownloadTargets
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	client := &http.Client{Timeout: 30 * time.Second}

	fmt.Printf("[NET-DL] Starting download loop (interval=%v-%v)\n", minInterval, maxInterval)

	for {
		select {
		case <-ctx.Done():
			fmt.Println("[NET-DL] Stop signal received. Exiting download worker.")
			return
		default:
		}

		target := targets[rng.Intn(len(targets))]
		reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target, nil)
		if err != nil {
			cancel()
			fmt.Println("[NET-DL] Request error:", err)
		} else {
			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("[NET-DL] Download error (%s): %v\n", target, err)
			} else {
				_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2*MiB))
				_ = resp.Body.Close()
				fmt.Printf("[NET-DL] %s -> %d\n", target, resp.StatusCode)
			}
			cancel()
		}

		wait := randomDuration(rng, minInterval, maxInterval)
		fmt.Printf("[NET-DL] Sleeping %v\n", wait)
		if !sleepWithContext(ctx, wait) {
			return
		}
	}
}
