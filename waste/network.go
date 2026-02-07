package waste

import (
	"context"
	"fmt"
	"time"

	"github.com/showwin/speedtest-go/speedtest"
)

func Network(interval time.Duration, connectionCount int) {
	fmt.Println("[NETWORK] Starting Network Waste Worker...")
	
	// FIX: Gunakan Client Instance, bukan GlobalDataManager (Deprecated)
	client := speedtest.New()

	user, err := client.FetchUserInfo()
	if err != nil {
		fmt.Println("[NETWORK] Warning: Cannot fetch user info:", err)
	} else {
		fmt.Printf("[NETWORK] IP: %s | ISP: %s\n", user.IP, user.Isp)
	}

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		
		// Fetch Server via Client
		serverList, err := client.FetchServers()
		if err != nil {
			fmt.Println("[NETWORK] Error fetching servers:", err)
			cancel()
			time.Sleep(1 * time.Minute)
			continue
		}

		targets, err := serverList.FindServer([]int{})
		if err != nil || len(targets) == 0 {
			fmt.Println("[NETWORK] No servers found.")
			cancel()
			time.Sleep(1 * time.Minute)
			continue
		}

		server := targets[0]
		fmt.Printf("[NETWORK] Testing against: %s (%s)\n", server.Sponsor, server.Name)

		// FIX: PingTest v1.7.9 menerima callback func, bukan context.
		// Kita pass nil saja kalau tidak butuh callback.
		err = server.PingTest(nil)
		
		if err != nil {
			fmt.Println("[NETWORK] Ping failed:", err)
		} else {
			dlCtx, dlCancel := context.WithTimeout(ctx, 45*time.Second)
			err = server.DownloadTestContext(dlCtx)
			dlCancel()
			if err != nil { fmt.Println("[NETWORK] Download warning:", err) }

			ulCtx, ulCancel := context.WithTimeout(ctx, 45*time.Second)
			err = server.UploadTestContext(ulCtx)
			ulCancel()
			if err != nil { fmt.Println("[NETWORK] Upload warning:", err) }

			fmt.Printf("[NETWORK] Result -> Ping: %d ms | DL: %.2f Mbps | UL: %.2f Mbps\n", 
				server.Latency.Milliseconds(), server.DLSpeed, server.ULSpeed)
		}

		// Reset via Client Manager jika ada, atau cukup loop ulang karena kita fetch baru tiap kali.
		// Di v1.7.9 reset global tidak diperlukan jika kita fetch fresh list.
		
		cancel()
		fmt.Printf("[NETWORK] Sleeping for %v...\n", interval)
		time.Sleep(interval)
	}
}
