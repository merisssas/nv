package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/merisssas/nv/controller"
	"github.com/merisssas/nv/waste"
)

const Version = "0.2.7-hardened-fix"

var (
	// Flags
	FlagCPUPercent = flag.Float64("cp", 0, "Target CPU load percent (e.g. 15-25)")
	FlagCPU        = flag.Duration("c", 0, "Control cycle interval (e.g. 1s). Used with -cp")
	FlagMemory     = flag.Int("m", 0, "GiB of memory waste (e.g. 6)")
	FlagMemoryPct  = flag.Float64("mp", 0, "Target memory usage percent (e.g. 20)")

	FlagAutoInterval   = flag.Duration("auto-interval", 2*time.Second, "Interval for auto resource checks")
	FlagAutoHysteresis = flag.Float64("auto-hyst", 2, "Auto pause/resume hysteresis percent")
	FlagAutoMode       = flag.Bool("auto", false, "Auto pause/resume CPU/Memory waste based on thresholds")

	FlagNetwork                = flag.Duration("n", 0, "Interval for network speed test (e.g. 45m)")
	FlagNetworkConnectionCount = flag.Int("t", 4, "Concurrent connections for speedtest")
	FlagBaseline               = flag.Bool("baseline", false, "Enable baseline workload (web server + cpu bump + memory touch + network faker)")
	FlagWebAddr                = flag.String("web-addr", "0.0.0.0", "Address for baseline web server")
	FlagWebPort                = flag.Int("web-port", 8080, "Port for baseline web server")

	FlagPriority = flag.Int("p", 0, "Process priority (0=Normal, 19=Lowest)")

	FlagDiskSize     = flag.Int("d", 0, "MiB of disk write per interval (e.g. 1024)")
	FlagDiskInterval = flag.Duration("di", 0, "Interval for disk write (e.g. 30m)")
	FlagDiskPath     = flag.String("path", ".", "Directory for disk waste (default: current dir)")
)

func main() {
	fmt.Println("NeverIdle", Version, "| Oracle Cloud Ampere Edition")
	fmt.Println("Platform:", runtime.GOOS, "/", runtime.GOARCH, "| Go:", runtime.Version())
	fmt.Println("Optimization: Thread-Safe, Anti-Swap, Low-Latency, Disk-IO, Graceful-Shutdown")
	fmt.Println("====================")

	flag.Parse()
	nothingEnabled := true

	// Setup Context untuk Graceful Shutdown (Ctrl+C / Docker Stop)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- 1. Priority Management ---
	if *FlagPriority != 0 {
		nothingEnabled = false
		if *FlagPriority == 666 {
			fmt.Println("[PRIORITY] Setting to Worst Priority (Background Mode)...")
			controller.SetWorstPriority()
		} else {
			fmt.Printf("[PRIORITY] Setting Priority to %d...\n", *FlagPriority)
			if err := controller.SetPriority(*FlagPriority); err != nil {
				fmt.Println("[PRIORITY] Warning: Failed to set priority:", err)
			}
		}
		fmt.Println("====================")
	}

	// --- 1b. Baseline Web Server ---
	if *FlagBaseline {
		nothingEnabled = false
		addr := fmt.Sprintf("%s:%d", *FlagWebAddr, *FlagWebPort)
		waste.StartWebServer(ctx, addr)
		fmt.Println("====================")
	}

	// --- 2. Memory Waste (PERBAIKAN DI SINI) ---
	var memAuto *memoryAutoController
	if *FlagMemory > 0 && *FlagMemoryPct > 0 {
		fmt.Println("[MAIN] Error: -m and -mp cannot be used together.")
		return
	}

	if *FlagMemoryPct > 0 {
		nothingEnabled = false
		target := *FlagMemoryPct
		if target < 0 {
			target = 0
		}
		if target > 100 {
			target = 100
		}

		fmt.Printf("[MAIN] Starting Auto Memory (Target: %.2f%%)\n", target)
		memAuto = &memoryAutoController{}
		memAuto.run(ctx, target, *FlagAutoInterval, *FlagAutoHysteresis)
		fmt.Println("====================")
	} else if *FlagMemory > 0 {
		nothingEnabled = false
		fmt.Printf("[MAIN] Starting Memory Waste: %d GiB\n", *FlagMemory)

		// Kita jalankan di background goroutine agar tidak memblokir program
		// Ikuti ctx utama agar bisa berhenti rapi saat shutdown
		go func() {
			if err := waste.Memory(ctx, *FlagMemory); err != nil {
				fmt.Printf("[MEMORY] Error: %v\n", err)
			}
		}()

		time.Sleep(1 * time.Second) // Beri waktu alokasi awal
		fmt.Println("====================")
	}

	if *FlagBaseline && *FlagMemory == 0 && *FlagMemoryPct == 0 {
		nothingEnabled = false
		fmt.Println("[MAIN] Starting Baseline Memory Touch (300-800 MiB every 5-15m)")
		go waste.MemoryBurst(ctx, 300, 800, 5*time.Minute, 15*time.Minute)
		fmt.Println("====================")
	}

	// --- 3. CPU Waste (Managed) ---
	var cpuBurner *waste.Burner // Simpan reference untuk distop nanti
	var cpuAuto *cpuAutoController
	var cpuBaseline *waste.Burner

	if *FlagCPU != 0 || *FlagCPUPercent > 0 {
		nothingEnabled = false
		interval := *FlagCPU
		if interval == 0 {
			interval = 1 * time.Second // Default cycle 1 detik agar stabil
		}

		ratio := 0.35 // Default burst center 35%
		if *FlagCPUPercent > 0 {
			ratio = *FlagCPUPercent / 100.0
		}

		// Safety Guard
		if ratio < 0 {
			ratio = 0
		}
		if ratio > 1 {
			ratio = 1
		}

		cfg := waste.Config{
			Interval: interval,
			Ratio:    ratio,
			Workers:  0, // 0 = Auto detect limit container
			AutoTune: *FlagAutoMode,
			BurstMin: 2 * time.Minute,
			BurstMax: 4 * time.Minute,
			RestMin:  2 * time.Minute,
			RestMax:  4 * time.Minute,
			// Fixed burst range to mimic real web server traffic.
			BurstRatioMin: 0.30,
			BurstRatioMax: 0.40,
			RestRatio:     0.01,
		}

		if *FlagAutoMode && *FlagCPUPercent > 0 {
			fmt.Printf("[MAIN] Starting Auto CPU (Cycle: %v, Burst: %.0f-%.0f%%, Rest: ~%.0f%%)\n",
				cfg.Interval,
				cfg.BurstRatioMin*100,
				cfg.BurstRatioMax*100,
				cfg.RestRatio*100,
			)
			cpuAuto = &cpuAutoController{cfg: cfg}
			cpuAuto.run(ctx, cfg.Ratio*100, *FlagAutoInterval, *FlagAutoHysteresis)
		} else {
			fmt.Printf("[MAIN] Starting CPU Waste (Cycle: %v, Burst: %.0f-%.0f%%, Rest: ~%.0f%%)\n",
				cfg.Interval,
				cfg.BurstRatioMin*100,
				cfg.BurstRatioMax*100,
				cfg.RestRatio*100,
			)

			var err error
			cpuBurner, err = waste.StartCPU(cfg)
			if err != nil {
				fmt.Printf("[MAIN] Error starting CPU waste: %v\n", err)
			}
		}
		fmt.Println("====================")
	}

	if *FlagBaseline && *FlagCPU == 0 && *FlagCPUPercent == 0 && !*FlagAutoMode {
		nothingEnabled = false
		cfg := waste.PatternConfig{
			Interval: 1 * time.Second,
			Phases: []waste.Phase{
				{
					Name:        "burst",
					DurationMin: 90 * time.Second,
					DurationMax: 180 * time.Second,
					RatioMin:    0.40,
					RatioMax:    0.70,
				},
				{
					Name:        "cooldown",
					DurationMin: 3 * time.Minute,
					DurationMax: 10 * time.Minute,
					RatioMin:    0.01,
					RatioMax:    0.05,
				},
				{
					Name:        "idle",
					DurationMin: 5 * time.Minute,
					DurationMax: 15 * time.Minute,
					RatioMin:    0.005,
					RatioMax:    0.03,
				},
			},
		}
		fmt.Println("[MAIN] Starting Baseline CPU bump pattern (40-70% for 90-180s; <5% cooldown/idle)")
		var err error
		cpuBaseline, err = waste.StartCPUPattern(cfg)
		if err != nil {
			fmt.Printf("[MAIN] Error starting baseline CPU pattern: %v\n", err)
		}
		fmt.Println("====================")
	}

	// --- 4. Network Waste ---
	if *FlagNetwork != 0 {
		nothingEnabled = false
		fmt.Printf("[MAIN] Starting Network Waste every %v (conns=%d)\n", *FlagNetwork, *FlagNetworkConnectionCount)
		go waste.Network(ctx, *FlagNetwork, *FlagNetworkConnectionCount)
		fmt.Println("====================")
	}

	if *FlagBaseline && *FlagNetwork == 0 {
		nothingEnabled = false
		fmt.Println("[MAIN] Starting Baseline Network Faker (2-12m, 3-7 domains)")
		go waste.NetworkFaker(ctx, 2*time.Minute, 12*time.Minute, 3, 7)
		fmt.Println("====================")
	}

	// --- 5. Disk Waste ---
	if *FlagDiskSize > 0 {
		nothingEnabled = false
		interval := *FlagDiskInterval
		if interval == 0 {
			interval = 30 * time.Minute
		}

		// Default path ke "." (Volume yang dimount)
		dir := *FlagDiskPath

		fmt.Printf("[MAIN] Starting Disk Waste: %d MiB every %v (Path: %s)\n", *FlagDiskSize, interval, dir)
		go waste.Disk(ctx, dir, *FlagDiskSize, interval)
		fmt.Println("====================")
	}

	if nothingEnabled {
		fmt.Println("[MAIN] No waste flags provided. Exiting...")
		flag.PrintDefaults()
		return
	}

	fmt.Println("[MAIN] All workers started. NeverIdle is running...")

	// --- BLOCK & WAIT FOR SHUTDOWN SIGNAL ---
	<-ctx.Done()

	fmt.Println("\n[MAIN] Shutdown signal received. Cleaning up...")

	// Matikan CPU Worker dengan sopan
	if cpuBurner != nil {
		fmt.Print("[MAIN] Stopping CPU Burner... ")
		cpuBurner.Stop()
	}
	if cpuAuto != nil {
		fmt.Print("[MAIN] Stopping Auto CPU... ")
		cpuAuto.Stop()
	}
	if cpuBaseline != nil {
		fmt.Print("[MAIN] Stopping Baseline CPU... ")
		cpuBaseline.Stop()
	}
	if memAuto != nil {
		fmt.Print("[MAIN] Stopping Auto Memory... ")
		memAuto.Stop()
	}

	fmt.Println("[MAIN] Goodbye.")
}
