package waste

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"strings"
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
	Ratio    float64       // Target penggunaan CPU (0.0 - 1.0) (legacy)
	Workers  int           // Jumlah worker (0 = otomatis sesuai container)
	BufSize  int           // Ukuran buffer enkripsi (default 32KB)
	Batch    int           // Cek waktu setiap N iterasi (default 1024)
	AutoTune bool          // Sesuaikan beban agar mendekati target total CPU (legacy)

	// Burst & Sleep settings
	BurstMin      time.Duration // Durasi minimum fase burst
	BurstMax      time.Duration // Durasi maksimum fase burst
	RestMin       time.Duration // Durasi minimum fase rest
	RestMax       time.Duration // Durasi maksimum fase rest
	BurstRatioMin float64       // Rasio minimum burst (0.0 - 1.0)
	BurstRatioMax float64       // Rasio maksimum burst (0.0 - 1.0)
	RestRatio     float64       // Rasio rest mendekati 0%
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

	workers := cfg.Workers
	if workers <= 0 {
		// Gunakan GOMAXPROCS(0) untuk menghormati limit container (Docker/K8s)
		workers = runtime.GOMAXPROCS(0)
		if workers <= 0 {
			workers = runtime.NumCPU() // Fallback ke CPU host jika gagal
		}
	}

	burstMinRatio := cfg.BurstRatioMin
	burstMaxRatio := cfg.BurstRatioMax
	if burstMinRatio <= 0 && burstMaxRatio <= 0 {
		burstMinRatio = 0.30
		burstMaxRatio = 0.40
	}
	if burstMinRatio < 0 {
		burstMinRatio = 0
	}
	if burstMaxRatio > 1 {
		burstMaxRatio = 1
	}
	if burstMaxRatio < burstMinRatio {
		burstMaxRatio = burstMinRatio
	}

	restRatio := cfg.RestRatio
	if restRatio < 0 {
		restRatio = 0
	}
	if restRatio == 0 {
		restRatio = 0.01
	}

	burstMin := cfg.BurstMin
	burstMax := cfg.BurstMax
	if burstMin <= 0 {
		burstMin = 2 * time.Minute
	}
	if burstMax <= 0 {
		burstMax = 4 * time.Minute
	}
	if burstMax < burstMin {
		burstMax = burstMin
	}

	restMin := cfg.RestMin
	restMax := cfg.RestMax
	if restMin <= 0 {
		restMin = 2 * time.Minute
	}
	if restMax <= 0 {
		restMax = 4 * time.Minute
	}
	if restMax < restMin {
		restMax = restMin
	}

	targetRatio := burstMinRatio
	ratioBits := &atomic.Uint64{}
	ratioBits.Store(math.Float64bits(targetRatio))

	fmt.Printf("[CPU] Starting %d workers (Burst: %.0f-%.0f%% for %v-%v, Rest: ~%.0f%% for %v-%v)\n",
		workers,
		burstMinRatio*100,
		burstMaxRatio*100,
		burstMin,
		burstMax,
		restRatio*100,
		restMin,
		restMax,
	)

	// Persiapkan source buffer untuk key & nonce
	// Setiap worker butuh Key(32) + Nonce(24) yang unik
	need := workers*(32+24) + cfg.BufSize
	source := make([]byte, need)
	if _, err := crand.Read(source); err != nil {
		return nil, fmt.Errorf("rand.Read failed: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	b := &Burner{cancel: cancel}
	b.running.Store(true)
	b.wg.Add(workers)

	go func() {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		for {
			burstRatio := randomRatio(rng, burstMinRatio, burstMaxRatio)
			burstDuration := randomDuration(rng, burstMin, burstMax)
			ratioBits.Store(math.Float64bits(burstRatio))
			fmt.Printf("[CPU] Burst phase: target %.0f%% for %v\n", burstRatio*100, burstDuration)
			if !sleepWithContext(ctx, burstDuration) {
				return
			}

			restRatioPhase := randomRatio(rng, 0, restRatio)
			restDuration := randomDuration(rng, restMin, restMax)
			ratioBits.Store(math.Float64bits(restRatioPhase))
			fmt.Printf("[CPU] Rest phase: target %.0f%% for %v\n", restRatioPhase*100, restDuration)
			if !sleepWithContext(ctx, restDuration) {
				return
			}
		}
	}()

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

				ratio := math.Float64frombits(ratioBits.Load())
				if ratio < 0 {
					ratio = 0
				} else if ratio > 1 {
					ratio = 1
				}

				busyDuration := time.Duration(float64(cfg.Interval) * ratio)
				idleDuration := cfg.Interval - busyDuration
				if idleDuration < 0 {
					idleDuration = 0
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

func randomDuration(rng *rand.Rand, min, max time.Duration) time.Duration {
	if max <= min {
		return min
	}
	delta := max - min
	return min + time.Duration(rng.Int63n(int64(delta)+1))
}

func randomRatio(rng *rand.Rand, min, max float64) float64 {
	if max <= min {
		return min
	}
	return min + rng.Float64()*(max-min)
}

func sleepWithContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// Stop menghentikan semua worker dengan rapi.
func (b *Burner) Stop() {
	if b == nil {
		return
	}
	// Pastikan hanya dipanggil sekali
	if b.running.CompareAndSwap(true, false) {
		b.cancel()  // Kirim sinyal cancel ke context
		b.wg.Wait() // Tunggu semua goroutine worker selesai
		fmt.Println("[CPU] Workers stopped.")
	}
}

type cpuStat struct {
	idle  uint64
	total uint64
}

func readCPUStat() (cpuStat, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuStat{}, err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return cpuStat{}, fmt.Errorf("empty /proc/stat")
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuStat{}, fmt.Errorf("unexpected /proc/stat format")
	}

	var total uint64
	var idle uint64
	for i := 1; i < len(fields); i++ {
		var v uint64
		if _, err := fmt.Sscanf(fields[i], "%d", &v); err != nil {
			return cpuStat{}, err
		}
		total += v
		if i == 4 || i == 5 { // idle + iowait
			idle += v
		}
	}
	return cpuStat{idle: idle, total: total}, nil
}

func cpuUsage(sample time.Duration) (float64, error) {
	s1, err := readCPUStat()
	if err != nil {
		return 0, err
	}
	time.Sleep(sample)
	s2, err := readCPUStat()
	if err != nil {
		return 0, err
	}
	deltaTotal := s2.total - s1.total
	deltaIdle := s2.idle - s1.idle
	if deltaTotal == 0 {
		return 0, fmt.Errorf("zero total delta")
	}
	usage := 1 - float64(deltaIdle)/float64(deltaTotal)
	if usage < 0 {
		usage = 0
	} else if usage > 1 {
		usage = 1
	}
	return usage, nil
}

// CPUUsage mengukur penggunaan CPU aktual (0.0 - 1.0) dalam interval sample.
func CPUUsage(sample time.Duration) (float64, error) {
	return cpuUsage(sample)
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
		Ratio:    0.20, // Target total CPU
		Workers:  0,    // Auto
		AutoTune: true,
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
