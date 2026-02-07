package waste

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"runtime"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

type WebServerConfig struct {
	Addr      string
	EnableTLS bool
	Domain    string
}

func StartWebServer(ctx context.Context, cfg WebServerConfig) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<!doctype html><html><head><title>NeverIdle</title></head><body><h1>System Status: OK</h1><p>Time: %s</p></body></html>", time.Now().Format(time.RFC3339))
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: mux,
	}
	var certManager *autocert.Manager
	if cfg.EnableTLS {
		if cfg.Domain == "" {
			fmt.Println("[WEB] TLS enabled but domain is empty; skipping TLS.")
		} else {
			certManager = &autocert.Manager{
				Cache:      autocert.DirCache("certs"),
				Prompt:     autocert.AcceptTOS,
				HostPolicy: autocert.HostWhitelist(cfg.Domain),
			}
			server.TLSConfig = &tls.Config{
				GetCertificate: certManager.GetCertificate,
				MinVersion:     tls.VersionTLS12,
			}
		}
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			fmt.Println("[WEB] Shutdown error:", err)
		}
	}()

	go func() {
		fmt.Printf("[WEB] Starting baseline web server on %s\n", cfg.Addr)
		if certManager != nil {
			go func() {
				if err := http.ListenAndServe(":80", certManager.HTTPHandler(nil)); err != nil {
					fmt.Println("[WEB] ACME HTTP server error:", err)
				}
			}()
			if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				fmt.Println("[WEB] Server error:", err)
			}
			return
		}
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println("[WEB] Server error:", err)
		}
	}()
}

func MemoryBurst(ctx context.Context, minMiB, maxMiB int, minInterval, maxInterval time.Duration) {
	if minMiB <= 0 || maxMiB <= 0 {
		fmt.Println("[MEM-BURST] Invalid memory burst sizes, skipping.")
		return
	}
	if maxMiB < minMiB {
		maxMiB = minMiB
	}
	if minInterval <= 0 {
		minInterval = 5 * time.Minute
	}
	if maxInterval < minInterval {
		maxInterval = minInterval
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		sizeMiB := minMiB
		if maxMiB > minMiB {
			sizeMiB = rng.Intn(maxMiB-minMiB+1) + minMiB
		}
		sizeBytes := sizeMiB * MiB
		fmt.Printf("[MEM-BURST] Allocating %d MiB\n", sizeMiB)

		buf := make([]byte, sizeBytes)
		for off := 0; off < len(buf); off += page {
			buf[off] = byte(off)
		}

		var checksum byte
		for off := 0; off < len(buf); off += page {
			checksum ^= buf[off]
		}
		if checksum == 0xFF {
			fmt.Fprint(io.Discard, "")
		}

		buf = nil
		runtime.GC()

		wait := randomDuration(rng, minInterval, maxInterval)
		fmt.Printf("[MEM-BURST] Released. Sleeping %v\n", wait)
		if !sleepWithContext(ctx, wait) {
			return
		}
	}
}

func NetworkFaker(ctx context.Context, minInterval, maxInterval time.Duration, minTargets, maxTargets int) {
	if minInterval <= 0 {
		minInterval = 2 * time.Minute
	}
	if maxInterval < minInterval {
		maxInterval = minInterval
	}
	if minTargets <= 0 {
		minTargets = 3
	}
	if maxTargets < minTargets {
		maxTargets = minTargets
	}

	targets := []string{
		"https://www.wikipedia.org/",
		"https://news.ycombinator.com/",
		"https://raw.githubusercontent.com/github/gitignore/main/Go.gitignore",
		"https://api.github.com/",
		"https://www.cloudflare.com/",
		"https://www.bbc.com/",
		"https://www.reuters.com/",
		"https://www.npr.org/",
		"https://www.nytimes.com/",
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	client := &http.Client{Timeout: 20 * time.Second}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		useDNS := rng.Intn(3) == 0
		if useDNS {
			fmt.Println("[NET-FAKER] DNS lookup whoami.cloudflare")
			dnsCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			_, err := net.DefaultResolver.LookupTXT(dnsCtx, "whoami.cloudflare")
			cancel()
			if err != nil {
				fmt.Println("[NET-FAKER] DNS error:", err)
			}
		} else {
			shuffled := append([]string{}, targets...)
			rng.Shuffle(len(shuffled), func(i, j int) {
				shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
			})
			count := minTargets
			if maxTargets > minTargets {
				count = rng.Intn(maxTargets-minTargets+1) + minTargets
			}
			if count > len(shuffled) {
				count = len(shuffled)
			}
			fmt.Printf("[NET-FAKER] Fetching %d targets\n", count)
			for i := 0; i < count; i++ {
				if !fetchURL(ctx, client, shuffled[i]) {
					break
				}
			}
		}

		wait := randomDuration(rng, minInterval, maxInterval)
		fmt.Printf("[NET-FAKER] Sleeping %v\n", wait)
		if !sleepWithContext(ctx, wait) {
			return
		}
	}
}

func fetchURL(ctx context.Context, client *http.Client, url string) bool {
	reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		fmt.Println("[NET-FAKER] Request error:", err)
		return true
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[NET-FAKER] Fetch error (%s): %v\n", url, err)
		return true
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 256*1024))
	fmt.Printf("[NET-FAKER] %s -> %d\n", url, resp.StatusCode)
	return true
}
