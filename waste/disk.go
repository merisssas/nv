package waste

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Disk writing worker
// path: lokasi menulis file (biasanya /tmp)
// sizeMiB: ukuran file dalam MiB
// interval: jeda antar penulisan
func Disk(ctx context.Context, path string, sizeMiB int, interval time.Duration) {
	if sizeMiB <= 0 {
		fmt.Println("[DISK] sizeMiB must be > 0. Skipping disk worker.")
		return
	}
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	const minInterval = 10 * time.Second
	if interval < minInterval {
		fmt.Printf("[DISK] Interval terlalu kecil (%v). Gunakan minimum %v untuk stabilitas.\n", interval, minInterval)
		interval = minInterval
	}

	const minFreeMiB = 256
	const maxUseFraction = 0.10

	fmt.Printf("[DISK] Starting Disk Waste Worker. Writing %d MiB to %s every %v\n", sizeMiB, path, interval)

	// Buffer data random
	data := make([]byte, 1024*1024) // 1 MiB chunk

	// Nama file temporary
	filePath := filepath.Join(path, "neveridle_waste.tmp")

	for {
		select {
		case <-ctx.Done():
			_ = os.Remove(filePath)
			fmt.Println("[DISK] Stop signal received. Exiting disk worker.")
			return
		default:
		}

		start := time.Now()

		freeMiB, err := getFreeMiB(path)
		if err != nil {
			fmt.Printf("[DISK] Error reading free space: %v\n", err)
			time.Sleep(interval)
			continue
		}

		if freeMiB <= minFreeMiB {
			fmt.Printf("[DISK] Low disk space: %d MiB free. Skipping write.\n", freeMiB)
			time.Sleep(interval)
			continue
		}

		maxWriteMiB := int(float64(freeMiB) * maxUseFraction)
		if maxWriteMiB < 1 {
			maxWriteMiB = 1
		}
		if maxWriteMiB > freeMiB-minFreeMiB {
			maxWriteMiB = freeMiB - minFreeMiB
		}

		writeMiB := sizeMiB
		if writeMiB > maxWriteMiB {
			writeMiB = maxWriteMiB
			fmt.Printf("[DISK] Capping write size to %d MiB (free=%d MiB).\n", writeMiB, freeMiB)
		}
		if writeMiB <= 0 {
			time.Sleep(interval)
			continue
		}

		// 1. Create File
		f, err := os.Create(filePath)
		if err != nil {
			fmt.Printf("[DISK] Error creating file: %v\n", err)
			time.Sleep(interval)
			continue
		}

		// 2. Write Data (Loop sesuai sizeMiB)
		// Kita tulis random data baru setiap chunk agar tidak dikompres oleh filesystem pintar (ZFS/Btrfs)
		for i := 0; i < writeMiB; i++ {
			rand.Read(data) // Isi random
			if _, err := f.Write(data); err != nil {
				fmt.Printf("[DISK] Error writing data: %v\n", err)
				break
			}
		}

		// 3. Sync (Paksa tulis dari RAM ke Disk Fisik)
		// Ini kunci agar grafis I/O di dashboard Oracle naik!
		f.Sync()
		f.Close()

		// 4. Delete File
		os.Remove(filePath)

		duration := time.Since(start)
		fmt.Printf("[DISK] Wrote & Deleted %d MiB in %v. Free=%d MiB. Sleeping...\n", writeMiB, duration, freeMiB)

		// Tidur sesuai interval
		time.Sleep(interval)
	}
}

func getFreeMiB(path string) (int, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	freeBytes := int64(stat.Bavail) * int64(stat.Bsize)
	if freeBytes < 0 {
		freeBytes = 0
	}
	return int(freeBytes / (1024 * 1024)), nil
}
