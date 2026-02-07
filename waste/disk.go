package waste

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Disk writing worker
// path: lokasi menulis file (biasanya /tmp)
// sizeMiB: ukuran file dalam MiB
// interval: jeda antar penulisan
func Disk(path string, sizeMiB int, interval time.Duration) {
	fmt.Printf("[DISK] Starting Disk Waste Worker. Writing %d MiB to %s every %v\n", sizeMiB, path, interval)

	// Buffer data random
	data := make([]byte, 1024*1024) // 1 MiB chunk
	
	// Nama file temporary
	filePath := filepath.Join(path, "neveridle_waste.tmp")

	for {
		start := time.Now()

		// 1. Create File
		f, err := os.Create(filePath)
		if err != nil {
			fmt.Printf("[DISK] Error creating file: %v\n", err)
			time.Sleep(interval)
			continue
		}

		// 2. Write Data (Loop sesuai sizeMiB)
		// Kita tulis random data baru setiap chunk agar tidak dikompres oleh filesystem pintar (ZFS/Btrfs)
		for i := 0; i < sizeMiB; i++ {
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
		fmt.Printf("[DISK] Wrote & Deleted %d MiB in %v. Sleeping...\n", sizeMiB, duration)

		// Tidur sesuai interval
		time.Sleep(interval)
	}
}
