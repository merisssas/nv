package waste

import (
	"crypto/rand"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/merisssas/nv/controller"
	"github.com/shirou/gopsutil/v4/cpu"
	"go.einride.tech/pid"
	"golang.org/x/crypto/chacha20"
)

// Global state untuk mencegah double-run
var (
	c              *pid.Controller
	currentMachine *machine
	globalMu       sync.Mutex
)

// CPUPercent mengatur target penggunaan CPU (0-100%) menggunakan PID Controller.
func CPUPercent(referencePercent float64) {
	globalMu.Lock()
	defer globalMu.Unlock()

	// 1. Matikan machine lama jika ada (Anti-Leak)
	if currentMachine != nil {
		currentMachine.Stop()
	}

	maxStep := 100000.0
	// Rate impact disesuaikan agar PID tidak terlalu agresif
	rateImpact := maxStep / 500 

	currentMachine = newMachine(maxStep)
	
	// Reset PID controller
	c = controller.RunPID(currentMachine, referencePercent, rateImpact, false)
}

type machine struct {
	mu              sync.Mutex
	runtimePeriod   time.Duration
	maxControlValue float64
	busyTime        time.Duration
	idleTime        time.Duration
	revolution      float64
	stop            chan struct{}
	wg              sync.WaitGroup
}

func newMachine(maxStep float64) *machine {
	// Periode kontrol 100ms memberikan keseimbangan responsivitas dan stabilitas
	period := 100 * time.Millisecond
	m := &machine{
		runtimePeriod:   period,
		maxControlValue: maxStep,
		idleTime:        period,
		stop:            make(chan struct{}),
	}

	n := runtime.GOMAXPROCS(0)
	m.wg.Add(n)
	for i := 0; i < n; i++ {
		go m.Run(i)
	}
	return m
}

// Stop memberikan sinyal berhenti ke semua worker dan menunggu mereka selesai
func (m *machine) Stop() {
	close(m.stop)
	m.wg.Wait()
}

func (m *machine) Run(id int) {
	defer m.wg.Done()

	// Buffer lokal untuk menghindari data race
	localBuf := make([]byte, 32*1024)
	if _, err := rand.Read(localBuf); err != nil {
		return
	}

	key := make([]byte, 32)
	nonce := make([]byte, 24)
	rand.Read(key)
	rand.Read(nonce)

	cipher, err := chacha20.NewUnauthenticatedCipher(key, nonce)
	if err != nil {
		log.Println("[PCPU] Error creating cipher:", err)
		return
	}

	for {
		// Cek sinyal stop sebelum memulai siklus baru
		select {
		case <-m.stop:
			return
		default:
		}

		// Ambil durasi busy/idle secara thread-safe
		m.mu.Lock()
		busy := m.busyTime
		idle := m.idleTime
		m.mu.Unlock()

		// Reset counter agar tidak panic (Counter Overflow)
		cipher.SetCounter(0)

		// FASE BUSY (CPU Burn)
		if busy > 0 {
			start := time.Now()
			// Loop tanpa batas, di-break manual
			for i := 0; ; i++ {
				cipher.XORKeyStream(localBuf, localBuf)
				
				// OPTIMASI: Cek waktu setiap 127 iterasi (Bitwise Check)
				// Mengurangi overhead syscall time.Now() secara drastis
				if i&127 == 0 {
					if time.Since(start) >= busy {
						break
					}
				}
			}
		}

		// FASE IDLE (Sleep)
		if idle > 0 {
			select {
			case <-m.stop:
				return
			case <-time.After(idle):
				// Lanjut ke siklus berikutnya
			}
		}
	}
}

// Measure mengukur penggunaan CPU aktual.
func (m *machine) Measure() float64 {
	// Menggunakan window 200ms (lebih cepat dari 1 detik)
	// agar PID controller mendapatkan feedback lebih cepat.
	percent, err := cpu.Percent(200*time.Millisecond, false)
	if err != nil || len(percent) == 0 {
		return 0
	}
	// Mengembalikan total usage system (sesuai monitoring Oracle)
	return percent[0]
}

// Control menerima output dari PID controller dan mengatur durasi busy/idle.
func (m *machine) Control(value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.revolution += value
	
	// Anti-Windup sederhana
	if m.revolution < 0 {
		m.revolution = 0
		if c != nil {
			c.Reset()
		}
	} else if m.revolution > m.maxControlValue {
		m.revolution = m.maxControlValue
	}

	// Hitung rasio duty cycle
	ratio := m.revolution / m.maxControlValue
	m.busyTime = time.Duration(float64(m.runtimePeriod.Nanoseconds()) * ratio)
	m.idleTime = m.runtimePeriod - m.busyTime
}
