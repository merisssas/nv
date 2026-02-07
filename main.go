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

	"github.com/mampirmasss/B/controller"
	"github.com/mampirmasss/B/waste"
)

const Version = "0.2.7-hardened-fix"

var (
	// Flags
	FlagCPUPercent = flag.Float64("cp", 0, "Target CPU load percent (e.g. 15-25)")
	FlagCPU        = flag.Duration("c", 0, "Control cycle interval (e.g. 1s). Used with -cp")
	FlagMemory     = flag.Int("m", 0, "GiB of memory waste (e.g. 6)")

	FlagNetwork                = flag.Duration("n", 0, "Interval for network speed test (e.g. 45m)")
	FlagNetworkConnectionCount = flag.Int("t", 4, "Concurrent connections for speedtest")

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

	// --- 2. Memory Waste (PERBAIKAN DI SINI) ---
	if *FlagMemory > 0 {
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

	// --- 3. CPU Waste (Managed) ---
	var cpuBurner *waste.Burner // Simpan reference untuk distop nanti

	if *FlagCPU != 0 || *FlagCPUPercent > 0 {
		nothingEnabled = false
		interval := *FlagCPU
		if interval == 0 {
			interval = 1 * time.Second // Default cycle 1 detik agar stabil
		}

		ratio := 0.20 // Default 20%
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
		}

		fmt.Printf("[MAIN] Starting CPU Waste (Cycle: %v, Target: %.2f%%)\n", cfg.Interval, cfg.Ratio*100)

		var err error
		cpuBurner, err = waste.StartCPU(cfg)
		if err != nil {
			fmt.Printf("[MAIN] Error starting CPU waste: %v\n", err)
		}
		fmt.Println("====================")
	}

	// --- 4. Network Waste ---
	if *FlagNetwork != 0 {
		nothingEnabled = false
		fmt.Printf("[MAIN] Starting Network Waste every %v (conns=%d)\n", *FlagNetwork, *FlagNetworkConnectionCount)
		go waste.Network(*FlagNetwork, *FlagNetworkConnectionCount)
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
		go waste.Disk(dir, *FlagDiskSize, interval)
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

	fmt.Println("[MAIN] Goodbye.")
}
