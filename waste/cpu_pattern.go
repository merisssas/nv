package waste

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/chacha20"
)

type Phase struct {
	Name        string
	DurationMin time.Duration
	DurationMax time.Duration
	RatioMin    float64
	RatioMax    float64
}

type PatternConfig struct {
	Interval time.Duration
	Workers  int
	BufSize  int
	Batch    int
	Phases   []Phase
}

func StartCPUPattern(cfg PatternConfig) (*Burner, error) {
	if cfg.Interval <= 0 {
		return nil, fmt.Errorf("interval must be > 0")
	}
	if len(cfg.Phases) == 0 {
		return nil, fmt.Errorf("phases must not be empty")
	}
	if cfg.BufSize <= 0 {
		cfg.BufSize = 32 * 1024
	}
	if cfg.Batch <= 0 {
		cfg.Batch = 1024
	}

	workers := cfg.Workers
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
		if workers <= 0 {
			workers = runtime.NumCPU()
		}
	}

	validated := make([]Phase, 0, len(cfg.Phases))
	for _, phase := range cfg.Phases {
		if phase.DurationMin <= 0 && phase.DurationMax <= 0 {
			return nil, fmt.Errorf("phase duration must be > 0")
		}
		if phase.DurationMin <= 0 {
			phase.DurationMin = phase.DurationMax
		}
		if phase.DurationMax < phase.DurationMin {
			phase.DurationMax = phase.DurationMin
		}
		if phase.RatioMin < 0 {
			phase.RatioMin = 0
		}
		if phase.RatioMax > 1 {
			phase.RatioMax = 1
		}
		if phase.RatioMax < phase.RatioMin {
			phase.RatioMax = phase.RatioMin
		}
		if phase.Name == "" {
			phase.Name = "phase"
		}
		validated = append(validated, phase)
	}

	ratioBits := &atomic.Uint64{}
	ratioBits.Store(math.Float64bits(validated[0].RatioMin))

	fmt.Printf("[CPU] Starting %d workers (Pattern phases: %d)\n", workers, len(validated))
	for _, phase := range validated {
		fmt.Printf("[CPU] Phase %s -> %.0f-%.0f%% for %v-%v\n",
			phase.Name,
			phase.RatioMin*100,
			phase.RatioMax*100,
			phase.DurationMin,
			phase.DurationMax,
		)
	}

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
			for _, phase := range validated {
				ratio := randomRatio(rng, phase.RatioMin, phase.RatioMax)
				duration := randomDuration(rng, phase.DurationMin, phase.DurationMax)
				ratioBits.Store(math.Float64bits(ratio))
				fmt.Printf("[CPU] Phase %s: target %.0f%% for %v\n", phase.Name, ratio*100, duration)
				if !sleepWithContext(ctx, duration) {
					return
				}
			}
		}
	}()

	for id := 0; id < workers; id++ {
		keyOff := id * 32
		nonceOff := workers*32 + id*24
		key := source[keyOff : keyOff+32]
		nonce := source[nonceOff : nonceOff+24]

		go func(id int, key, nonce []byte) {
			defer b.wg.Done()

			buf := make([]byte, cfg.BufSize)
			copy(buf, source[workers*(32+24):workers*(32+24)+cfg.BufSize])

			timer := time.NewTimer(0)
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			defer timer.Stop()

			for {
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

				if busyDuration > 0 {
					c, err := chacha20.NewUnauthenticatedCipher(key, nonce)
					if err != nil {
						return
					}

					deadline := time.Now().Add(busyDuration)
					for {
						for i := 0; i < cfg.Batch; i++ {
							c.XORKeyStream(buf, buf)
						}
						if time.Now().After(deadline) {
							break
						}
						select {
						case <-ctx.Done():
							return
						default:
						}
					}
				}

				if idleDuration > 0 {
					timer.Reset(idleDuration)
					select {
					case <-ctx.Done():
						return
					case <-timer.C:
					}
				}
			}
		}(id, key, nonce)
	}

	return b, nil
}
