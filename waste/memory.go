package waste

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	KiB  = 1024
	MiB  = 1024 * KiB
	GiB  = 1024 * MiB
	page = 4096
)

var (
	mu      sync.Mutex
	Buffers [][]byte
	held    atomic.Int64
)

// Memory: BENAR-BENAR mengisi RAM fisik (RSS) dengan page-touch.
// Cgroup v2 aware (Docker/systemd scope aware).
func Memory(ctx context.Context, targetGiB int) error {
	if targetGiB <= 0 {
		return errors.New("targetGiB must be > 0")
	}
	return MemoryBytes(ctx, int64(targetGiB)*int64(GiB))
}

func MemoryBytes(ctx context.Context, targetBytes int64) error {
	if targetBytes <= 0 {
		return errors.New("targetBytes must be > 0")
	}

	// headroom supaya tidak langsung OOM
	const minHeadroom = 300 * MiB
	const headroomFraction = 0.20
	const maxUseFraction = 0.60

	// chunk kecil agar spike lebih halus
	const baseChunk = 16 * MiB

	fmt.Printf("[Memory] Targeting %.2f GiB Physical RAM (page-touch)\n",
		float64(targetBytes)/float64(GiB))

	// log cgroup setiap 2 detik
	logTicker := time.NewTicker(2 * time.Second)
	defer logTicker.Stop()

	for held.Load() < targetBytes {
		select {
		case <-ctx.Done():
			Release()
			return ctx.Err()
		case <-logTicker.C:
			printCgroupUsage()
		default:
		}

		safeFree, _ := getSafeFreeMemory()
		headroom := int64(float64(safeFree) * headroomFraction)
		if headroom < minHeadroom {
			headroom = minHeadroom
		}

		if safeFree < headroom {
			fmt.Printf("[Memory] Stop: safeFree=%.2f MiB < headroom=%.2f MiB. Held=%.2f GiB\n",
				float64(safeFree)/float64(MiB),
				float64(headroom)/float64(MiB),
				float64(held.Load())/float64(GiB),
			)
			break
		}

		// batasi target agar tidak “menghabiskan” free memory
		maxAllowed := int64(float64(safeFree) * maxUseFraction)
		if maxAllowed < 0 {
			maxAllowed = 0
		}
		cappedTarget := targetBytes
		if cappedTarget > maxAllowed {
			cappedTarget = maxAllowed
		}
		if held.Load() >= cappedTarget {
			fmt.Printf("[Memory] Stop: reached capped target %.2f GiB (held=%.2f GiB)\n",
				float64(cappedTarget)/float64(GiB),
				float64(held.Load())/float64(GiB),
			)
			break
		}

		chunk := int64(baseChunk)

		remain := cappedTarget - held.Load()
		if chunk > remain {
			chunk = remain
		}

		maxAlloc := safeFree - headroom
		if chunk > maxAlloc {
			chunk = maxAlloc
		}
		if chunk <= 0 {
			break
		}

		if chunk > int64(^uint(0)>>1) {
			return errors.New("chunk too large for this arch")
		}

		// ALLOC
		b := make([]byte, int(chunk))

		// 🔥 PAKSA RAM FISIK: touch setiap page 4KB
		// Ini yang bikin VmRSS + memory.current naik beneran.
		for off := 0; off < len(b); off += page {
			b[off] = 1
		}

		// (opsional tapi makin “keras”): tulis 1 byte per 64KB juga
		// biar makin sulit di-reclaim pada sebagian kernel:
		for off := 0; off < len(b); off += 64 * KiB {
			b[off] ^= 0x5A
		}

		mu.Lock()
		Buffers = append(Buffers, b)
		mu.Unlock()

		held.Add(chunk)

		// kecilin spike CPU
		time.Sleep(10 * time.Millisecond)
	}

	runtime.GC()
	fmt.Printf("[Memory] Allocation complete. Holding ~%.2f GiB\n",
		float64(held.Load())/float64(GiB))
	printCgroupUsage()

	keepAlive(ctx)
	return nil
}

// keepAlive: supaya benar-benar “nempel”, kita touch page secara periodik.
// Kalau cuma sentuh 1 byte, kernel bisa dinginkan sisanya.
func keepAlive(ctx context.Context) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()

	// jangan sentuh semua buffer tiap tick (CPU bisa tinggi)
	const buffersPerTick = 2
	const maxPagesPerBuffer = 64

	var rot int
	for {
		select {
		case <-ctx.Done():
			Release()
			return
		case <-t.C:
			mu.Lock()
			n := len(Buffers)
			if n == 0 {
				mu.Unlock()
				continue
			}

			for k := 0; k < buffersPerTick; k++ {
				i := rot % n
				rot++

				b := Buffers[i]

				// Touch sebagian page agar tetap “nempel” tanpa membebani CPU
				stride := len(b) / maxPagesPerBuffer
				if stride < page {
					stride = page
				}
				for off := 0; off < len(b); off += stride {
					b[off] ^= 1
				}
			}
			mu.Unlock()

			printCgroupUsage()
		}
	}
}

func Release() {
	mu.Lock()
	Buffers = nil
	mu.Unlock()
	held.Store(0)
	runtime.GC()
	fmt.Println("[Memory] Released all memory.")
}

// =======================
// HOST + CGROUP HELPERS
// =======================

func getSafeFreeMemory() (int64, error) {
	hostFree, err := getHostMemAvailable()
	if err != nil {
		return 0, err
	}
	cgFree, err := getCgroupFree()
	if err != nil {
		return hostFree, nil
	}
	if cgFree < hostFree {
		return cgFree, nil
	}
	return hostFree, nil
}

func getHostMemAvailable() (int64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			kb, err := strconv.ParseInt(fields[1], 10, 64)
			return kb * 1024, err
		}
	}
	return 0, errors.New("no MemAvailable")
}

func getHostMemTotalAvailable() (int64, int64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	var total int64
	var available int64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			fields := strings.Fields(line)
			kb, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0, 0, err
			}
			total = kb * 1024
		case strings.HasPrefix(line, "MemAvailable:"):
			fields := strings.Fields(line)
			kb, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0, 0, err
			}
			available = kb * 1024
		}
		if total > 0 && available > 0 {
			return total, available, nil
		}
	}
	if err := sc.Err(); err != nil {
		return 0, 0, err
	}
	return 0, 0, errors.New("no MemTotal/MemAvailable")
}

type MemoryStats struct {
	UsedBytes  int64
	TotalBytes int64
	Source     string
}

func MemoryUsage() (MemoryStats, error) {
	base, err := getSelfCgroupBase()
	if err == nil {
		usageRaw, err := os.ReadFile(base + "/memory.current")
		if err == nil {
			maxRaw, err := os.ReadFile(base + "/memory.max")
			if err == nil {
				usage, err := strconv.ParseInt(strings.TrimSpace(string(usageRaw)), 10, 64)
				if err != nil {
					return MemoryStats{}, err
				}

				maxStr := strings.TrimSpace(string(maxRaw))
				if maxStr != "max" {
					limit, err := strconv.ParseInt(maxStr, 10, 64)
					if err != nil {
						return MemoryStats{}, err
					}
					if limit > 0 {
						return MemoryStats{
							UsedBytes:  usage,
							TotalBytes: limit,
							Source:     "cgroup",
						}, nil
					}
				}
			}
		}
	}

	total, available, err := getHostMemTotalAvailable()
	if err != nil {
		return MemoryStats{}, err
	}
	used := total - available
	if used < 0 {
		used = 0
	}
	return MemoryStats{
		UsedBytes:  used,
		TotalBytes: total,
		Source:     "host",
	}, nil
}

// cgroup v2: ambil base dari /proc/self/cgroup (FIX untuk docker scope)
func getCgroupFree() (int64, error) {
	base, err := getSelfCgroupBase()
	if err != nil {
		return 0, err
	}
	usageRaw, err := os.ReadFile(base + "/memory.current")
	if err != nil {
		return 0, err
	}
	maxRaw, err := os.ReadFile(base + "/memory.max")
	if err != nil {
		return 0, err
	}

	usage, err := strconv.ParseInt(strings.TrimSpace(string(usageRaw)), 10, 64)
	if err != nil {
		return 0, err
	}

	maxStr := strings.TrimSpace(string(maxRaw))
	if maxStr == "max" {
		return 1 << 62, nil
	}

	limit, err := strconv.ParseInt(maxStr, 10, 64)
	if err != nil {
		return 0, err
	}

	free := limit - usage
	if free < 0 {
		free = 0
	}
	return free, nil
}

func getSelfCgroupBase() (string, error) {
	raw, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		parts := strings.SplitN(ln, ":", 3)
		if len(parts) < 3 {
			continue
		}
		rel := strings.TrimSpace(parts[2])
		if rel == "" {
			rel = "/"
		}
		if !strings.HasPrefix(rel, "/") {
			rel = "/" + rel
		}
		return "/sys/fs/cgroup" + rel, nil
	}
	return "", errors.New("no cgroup v2 entry found")
}

func printCgroupUsage() {
	base, err := getSelfCgroupBase()
	if err != nil {
		return
	}
	curRaw, err1 := os.ReadFile(base + "/memory.current")
	maxRaw, err2 := os.ReadFile(base + "/memory.max")
	if err1 != nil || err2 != nil {
		return
	}

	cur, _ := strconv.ParseInt(strings.TrimSpace(string(curRaw)), 10, 64)
	maxStr := strings.TrimSpace(string(maxRaw))

	if maxStr != "max" {
		max, _ := strconv.ParseInt(maxStr, 10, 64)
		fmt.Printf("[cgroup] memory.current=%.2f GiB / max=%.2f GiB\n",
			float64(cur)/float64(GiB),
			float64(max)/float64(GiB),
		)
	} else {
		fmt.Printf("[cgroup] memory.current=%.2f GiB (max=unlimited)\n",
			float64(cur)/float64(GiB),
		)
	}
}
