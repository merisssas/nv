package controller

import (
	"fmt"
	"time"

	"go.einride.tech/pid"
)

// Sampling interval 1 detik adalah standar industri untuk monitoring infrastruktur
// seperti Oracle Cloud Metric. Jangan diubah terlalu cepat agar grafik tidak "jittery".
const samplingInterval = time.Second

type Device interface {
	Control(value float64)
	Measure() float64
}

func RunPID(
	device Device,
	referenceSignal float64, // Bisa input 0.2 (desimal) atau 20 (persen)
	rateImpact float64,
	debug bool,
) *pid.Controller {
	
	// 1. Normalisasi Input Cerdas
	// Oracle biasanya melihat usage dalam persen (0-100).
	// Kita pastikan referenceSignal selalu dalam format Persen (0-100).
	if referenceSignal <= 1.0 && referenceSignal > 0 {
		// Jika user input 0.2, kita anggap 20%
		referenceSignal *= 100
	}

	// Safety check: Clamp value antara 0 sampai 100
	if referenceSignal < 0 {
		referenceSignal = 0
	} else if referenceSignal > 100 {
		fmt.Printf("[PID] Warning: Target %.2f%% is too high, capping at 100%%\n", referenceSignal)
		referenceSignal = 100
	}

	if debug {
		fmt.Printf("[PID] Controller started. Target: %.2f%%\n", referenceSignal)
	}

	// 2. Setup PID Controller
	// IntegralGain 1.0 cukup agresif, tapi oke untuk CPU load.
	// Kita tambahkan limitasi logic nanti di Device interface (pcpu.go).
	c := &pid.Controller{
		Config: pid.ControllerConfig{
			ProportionalGain: rateImpact,
			IntegralGain:     0.5, // Sedikit diturunkan agar lebih stabil (kurang osilasi)
			DerivativeGain:   0.0, // D=0 karena CPU noise tinggi, Derivative justru bikin tidak stabil
		},
	}

	// 3. Goroutine dengan Ticker (PERBAIKAN UTAMA)
	go func() {
		// Gunakan Ticker agar loop berjalan TEPAT setiap 'samplingInterval' (1 detik).
		// Ini memperbaiki matematika PID yang rusak di kode lama.
		ticker := time.NewTicker(samplingInterval)
		defer ticker.Stop()

		for range ticker.C {
			// A. Ukur kondisi saat ini (dari pcpu.go)
			actualSignal := device.Measure()

			// B. Hitung koreksi PID
			// Delta time (dt) sekarang benar-benar sesuai samplingInterval
			c.Update(pid.ControllerInput{
				ReferenceSignal:  referenceSignal,
				ActualSignal:     actualSignal,
				SamplingInterval: samplingInterval,
			})

			// C. Terapkan kontrol (Output PID)
			// Sinyal kontrol dikirim kembali ke device untuk menyesuaikan beban
			device.Control(c.State.ControlSignal)

			if debug {
				// Format log yang lebih rapi untuk monitoring
				fmt.Printf("[PID] Target: %.1f%% | Actual: %.1f%% | ControlOut: %.4f\n", 
					referenceSignal, actualSignal, c.State.ControlSignal)
			}
		}
	}()

	return c
}
