package waste

import (
	"context"
	"crypto/rand"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/chacha20"
)

// Burner mengontrol worker CPU waste.
type Burner struct {
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running atomic.Bool
}

// Config konfigurasi untuk CPU waste.
type Config struct {
	Interval time.Duration // Total durasi satu siklus (Busy + Idle)
	Ratio    float64       // Target penggunaan CPU (0.0 - 1.0)
	Workers  int           // Jumlah worker (0 = otomatis sesuai container)
	BufSize  int           // Ukuran buffer enkripsi (default 32KB)
	Batch    int           // Cek waktu setiap N iterasi (default 1024)
}

// StartCPU memulai worker CPU waste dengan konfigurasi yang diberikan.
func StartCPU(cfg Config) (*Burner, error) {
	if cfg.Interval <= 0 {
		return nil, fmt.Errorf("interval must be > 0")
	}
	if cfg.Ratio < 0 || cfg.Ratio > 1 {
		return nil, fmt.Errorf("ratio must be between 0.0 and 1.0")
	}
	if cfg.BufSize <= 0 {
		cfg.BufSize = 32 * 1024
	}
	if cfg.Batch <= 0 {
		cfg.Batch = 1024 // Batch besar = overhead minim
	}

	// Hitung durasi Busy dan Idle berdasarkan rasio
	busyDuration := time.Duration(float64(cfg.Interval) * cfg.Ratio)
	idleDuration := cfg.Interval - busyDuration

	workers := cfg.Workers
	if workers <= 0 {
		// Gunakan GOMAXPROCS(0) untuk menghormati limit container (Docker/K8s)
		workers = runtime.GOMAXPROCS(0)
		if workers <= 0 {
			workers = runtime.NumCPU() // Fallback ke CPU host jika gagal
		}
	}

	fmt.Printf("[CPU] Starting %d workers (Busy: %v, Idle: %v)\n", workers, busyDuration, idleDuration)

	// Persiapkan source buffer untuk key & nonce
	// Setiap worker butuh Key(32) + Nonce(24) yang unik
	need := workers*(32+24) + cfg.BufSize
	source := make([]byte, need)
	if _, err := rand.Read(source); err != nil {
		return nil, fmt.Errorf("rand.Read failed: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	b := &Burner{cancel: cancel}
	b.running.Store(true)
	b.wg.Add(workers)

	for id := 0; id < workers; id++ {
		// Slice key/nonce unik untuk setiap worker (tanpa overlap)
		keyOff := id * 32
		nonceOff := workers*32 + id*24
		key := source[keyOff : keyOff+32]
		nonce := source[nonceOff : nonceOff+24]

		go func(id int, key, nonce []byte) {
			defer b.wg.Done()

			// Buffer lokal per worker
			buf := make([]byte, cfg.BufSize)
			copy(buf, source[workers*(32+24):workers*(32+24)+cfg.BufSize])

			// Reuse timer untuk idle phase (Hemat GC)
			timer := time.NewTimer(0)
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			defer timer.Stop()

			for {
				// Cek sinyal stop di awal loop
				select {
				case <-ctx.Done():
					return
				default:
				}

				// --- BUSY PHASE ---
				if busyDuration > 0 {
					// Re-init cipher setiap siklus busy (Security & Correctness)
					// Mencegah counter overflow jika jalan sangat lama
					c, err := chacha20.NewUnauthenticatedCipher(key, nonce)
					if err != nil {
						return // Harusnya tidak terjadi
					}

					deadline := time.Now().Add(busyDuration)
					for {
						// Batch processing: Cek waktu setiap N iterasi
						// Mengurangi overhead syscall time.Now() secara drastis
						for i := 0; i < cfg.Batch; i++ {
							c.XORKeyStream(buf, buf)
						}

						if time.Now().After(deadline) {
							break
						}

						// Cek stop signal sesekali di tengah kesibukan
						select {
						case <-ctx.Done():
							return
						default:
						}
					}
				}

				// --- IDLE PHASE ---
				if idleDuration > 0 {
					timer.Reset(idleDuration)
					select {
					case <-ctx.Done():
						return
					case <-timer.C:
						// Lanjut ke siklus berikutnya
					}
				}
			}
		}(id, key, nonce)
	}

	return b, nil
}

// Stop menghentikan semua worker dengan rapi.
func (b *Burner) Stop() {
	if b == nil {
		return
	}
	// Pastikan hanya dipanggil sekali
	if b.running.CompareAndSwap(true, false) {
		b.cancel()    // Kirim sinyal cancel ke context
		b.wg.Wait()   // Tunggu semua goroutine worker selesai
		fmt.Println("[CPU] Workers stopped.")
	}
}

// Wrapper lama untuk kompatibilitas dengan main.go (Interval Mode)
func CPU(interval time.Duration) {
	// Default: 20% Load (Busy 20ms, Idle 80ms jika interval 100ms)
	// Namun karena func CPU() lama menerima interval sebagai "jeda tidur",
	// kita asumsikan interval adalah waktu IDLE.
	// Kita set waktu BUSY statis misal 50ms untuk memberikan beban.
	
	// Agar lebih fleksibel, kita pakai mode rasio 20%
	// Total siklus = interval * 5 (Contoh: Interval 100ms -> Siklus 500ms -> Busy 100ms)
	// Ini estimasi kasar untuk backward compatibility.
	
	// Tapi lebih baik kita set rasio fix 20% dengan interval 1 detik agar stabil.
	cfg := Config{
		Interval: 1 * time.Second,
		Ratio:    0.20, // 20% CPU
		Workers:  0,    // Auto
	}
	
	burner, err := StartCPU(cfg)
	if err != nil {
		fmt.Println("[CPU] Error starting:", err)
		return
	}
	
	// Block forever karena fungsi CPU() lama diharapkan blocking
	select {}
	
	// Unreachable in this context, but good practice
	burner.Stop() 
}
