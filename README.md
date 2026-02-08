# NeverIdle — Prestige Workload Generator for Always-Active VPS

**Repository:** https://github.com/merisssas/nv

NeverIdle is a lightweight utility that keeps VMs/VPS instances active using controlled CPU, memory, disk I/O, and network patterns. It is designed for environments at risk of being reclaimed due to idleness and for maintaining a realistic “activity footprint” without interfering with primary workloads.

> **Tagline:** _Prestige waster_ — stable, measured, and flexible across all machine sizes.

---

## Table of Contents

- [Overview](#overview)
- [Key Advantages](#key-advantages)
- [Core Features](#core-features)
- [Quick Start](#quick-start)
- [Feature Matrix](#feature-matrix)
- [Installation](#installation)
- [Usage](#usage)
- [Profiles & Presets](#profiles--presets)
- [Configuration (Flags)](#configuration-flags)
- [Full Tutorial](#full-tutorial)
- [Docker (Build, Run, Compose)](#docker-build-run-compose)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)
- [Security & Notes](#security--notes)

---

## Overview

NeverIdle combines CPU burner, memory allocator, disk writer, and network activity in a single cross-platform binary (Linux/macOS/Windows). It offers a realistic **Baseline Profile** as well as a system-adaptive **Smart Profile**. All workers include graceful shutdown, lock file protection, and priority controls to avoid impacting critical services.

---

## Key Advantages

- **All-in-one**: CPU, memory, disk, and network in one binary.
- **Adaptive**: auto pause/resume with hysteresis for CPU/memory.
- **Realistic**: baseline profile with natural traffic/memory patterns.
- **Safe**: graceful shutdown, lock file, and cgroup-aware limits.
- **Portable**: Linux/macOS/Windows.
- **Operational**: suitable for small to large VPS instances.

---

## Core Features

- CPU target load with burst/rest pattern
- Memory allocation (GiB) **or** percentage target
- Scheduled disk I/O bursts
- Periodic network speedtests or lightweight downloads
- Baseline web server + ACME TLS
- Maintenance scheduler (random compute/network/memory)
- Smart profile: auto-detect specs + CPU/memory targets
- Priority tuning + lock file

---

## Quick Start

### 1) Smart Mode (recommended)
Targets **20–40% CPU** and **~30% memory**, displays system specs, and performs light network downloads every 5–10 minutes.

```bash
./NeverIdle -smart
```

### 2) Baseline Mode (realistic footprint)

```bash
./NeverIdle -baseline
```

### 3) Minimal CPU + Memory + Network

```bash
./NeverIdle -cp 20 -c 1s -m 2 -n 4h
```

---

## Feature Matrix

| Component | Mode | Description | Default | Notes |
|---|---|---|---|---|
| CPU | `-cp` + `-c` | Target CPU with burst/rest | 35% target (when `-cp` used) | Auto pause/resume with `-auto` |
| CPU | `-baseline` | Burst 40–70% + cooldown/idle | Enabled in baseline | Realistic traffic footprint |
| CPU | `-smart` | 20–40% CPU with auto pause/resume | Enabled in smart | Safe on most VPS |
| Memory | `-m` | Fixed GiB allocation | Off | Cannot combine with `-mp` |
| Memory | `-mp` | Target memory percentage | Off | Cgroup-aware |
| Memory | `-smart` | ~30% memory target | Enabled in smart | Auto-adjust |
| Disk | `-d` + `-di` | Scheduled disk write burst | Off | `-path` for target dir |
| Network | `-n` | Periodic speedtest | Off | Requires internet access |
| Network | `-network-mode` | `speedtest`, `speedtest-random`, `speedtest-worst`, `download` | `speedtest` | `download` fetches small files |
| Web | `-baseline` | Dummy web server | Port 8080 | TLS via ACME |
| Scheduler | `-maintenance` | Random compute/network/memory | Off | Natural patterns |

---

## Installation

### Option A: Download release binary
1. Open Releases: https://github.com/merisssas/nv/releases
2. Download the binary for your architecture (amd64/arm64).
3. Make it executable:

```bash
chmod +x NeverIdle
```

### Option B: Build from source

```bash
git clone https://github.com/merisssas/nv.git
cd nv
go build -o NeverIdle
```

### Option C: Docker

```bash
# ARM (default)
sudo docker build -t neveridle:latest .
# AMD64
sudo docker build --build-arg ARCH=amd64 -t neveridle:latest .
```

---

## Usage

> **Note:** Percent values use whole numbers (e.g. `20` means 20%).

```bash
./NeverIdle -smart
```

At startup, each worker runs an initial cycle so you can immediately observe the effect.

---

## Profiles & Presets

### 1) Smart Profile (recommended)
- Prints CPU and memory specs
- CPU target 20–40% (burst pattern)
- Memory target ~30%
- Auto pause/resume enabled
- **Note:** Smart mode enables CPU + memory + light network download. Disk I/O runs only if you pass `-d` (and optionally `-di`/`-path`).

```bash
./NeverIdle -smart
```

### 2) Baseline Profile (realistic)
- Dummy web server (`/` and `/healthz`)
- CPU burst 40–70% with cooldown/idle
- Memory touch 300–800 MiB every 5–15 minutes
- Network faker (DNS + light HTTP)

```bash
./NeverIdle -baseline
```

### 3) Custom Profile (manual)

```bash
./NeverIdle -cp 25 -c 1s -mp 60 -n 6h -t 4 -d 1024 -di 1h -path /var/tmp
```

---

## Configuration (Flags)

### Smart Profile
| Flag | Description | Default |
|---|---|---|
| `-smart` | Detect specs + target 20–40% CPU and ~30% memory | Off |

### CPU
| Flag | Description | Default |
|---|---|---|
| `-cp` | Target CPU percent | 35% (if `-cp` used) |
| `-c` | CPU control interval | 1s |
| `-auto` | Auto pause/resume CPU | Off |
| `-auto-interval` | Auto check interval | 2s |
| `-auto-hyst` | Hysteresis percent | 2 |

### Memory
| Flag | Description | Default |
|---|---|---|
| `-m` | Fixed memory GiB | Off |
| `-mp` | Target memory percent | Off |

> `-m` and `-mp` cannot be used together.

### Network
| Flag | Description | Default |
|---|---|---|
| `-n` | Speedtest interval (or fixed interval for `download`) | Off |
| `-t` | Concurrent connections | 4 |
| `-network-mode` | `speedtest`, `speedtest-random`, `speedtest-worst`, `download` | `speedtest` |
| `-n-min` | Minimum interval for `download` | 5m (smart) |
| `-n-max` | Maximum interval for `download` | 10m (smart) |

### Baseline Web Server
| Flag | Description | Default |
|---|---|---|
| `-baseline` | Enable baseline profile | Off |
| `-web-addr` | Bind address | 0.0.0.0 |
| `-web-port` | Web server port | 8080 |
| `-web-tls` | Automatic ACME TLS | Off |
| `-web-domain` | TLS domain (required for TLS) | "" |

### Disk
| Flag | Description | Default |
|---|---|---|
| `-d` | Disk write MiB per interval | Off |
| `-di` | Disk write interval | 30m |
| `-path` | Target folder for disk writes | current directory |

### Maintenance + Safety
| Flag | Description | Default |
|---|---|---|
| `-maintenance` | Randomized scheduler | Off |
| `-heartbeat-url` | Heartbeat URL | Off |
| `-lock-file` | Lock file path | /tmp/neveridle.lock |
| `-p` | Priority (0=normal, 19=lowest, 666=background) | 0 |

---

## Full Tutorial

### Web Hosting Mode (Baseline Web Server)

This mode runs a lightweight web server to keep your VPS “visible” at the HTTP layer. It is useful for:

- basic health checks/load balancers,
- monitoring endpoints (e.g. `/healthz`),
- platforms that require an active web service.

**Default behavior:**
- Binds to `0.0.0.0:8080`.
- Primary endpoints: `/` and `/healthz`.
- Only runs when `-baseline` is enabled.

**Basic usage (direct binary):**

```bash
./NeverIdle -baseline -web-port 8080
```

**Domain + automatic TLS (ACME, direct binary):**

```bash
./NeverIdle -baseline -web-tls -web-domain example.com
```

**Docker (manual):**

```bash
docker run -d --name neveridle \
  --restart unless-stopped \
  -p 8080:8080 \
  ghcr.io/merisssas/nv:latest -baseline -web-port 8080
```

**Docker + TLS (manual):**

```bash
docker run -d --name neveridle \
  --restart unless-stopped \
  -p 80:80 -p 443:443 \
  ghcr.io/merisssas/nv:latest -baseline -web-tls -web-domain example.com
```

**Compose:**

```yaml
services:
  neveridle:
    image: ghcr.io/merisssas/nv:latest
    container_name: neveridle
    restart: unless-stopped
    ports:
      - "8080:8080"
    command: ["-baseline", "-web-port", "8080"]
```

**Important notes:**
- Ensure ports 80/443 are open for ACME verification.
- If another web server is running (Nginx/Caddy), use a different port (e.g. `-web-port 8081`) and proxy it.
- This is a **dummy** web service for activity footprint only; it does not serve your application.

### Tuning Tips

- If CPU usage is below 20–25%, increase the **burst percentage** (`-cp`) or adjust burst durations in `main.go`.
- For large VMs (e.g. 4 OCPU / 24 GB), ensure memory waste is not too low: use `-m 6` or adjust `-mp`.
- For light network activity, use `-network-mode download` for periodic small downloads (e.g. 5–10 minutes).

### 1) Keep VM active (minimal)

```bash
./NeverIdle -cp 20 -c 1s -m 2 -n 4h
```

**Expected output:** Stable CPU, fixed memory allocation, and speedtest every 4 hours.

---

### 2) Smart mode for any VPS

```bash
./NeverIdle -smart
```

**Benefit:** automatic system detection with safe 20–40% CPU load.

---

### 3) Baseline mode (realistic profile)

```bash
./NeverIdle -baseline
```

Enable TLS:

```bash
./NeverIdle -baseline -web-tls -web-domain example.com
```

---

### 4) Auto pause/resume CPU (adaptive)

```bash
./NeverIdle -cp 25 -c 1s -auto -auto-interval 2s -auto-hyst 3
```

---

### 5) Auto pause/resume memory

```bash
./NeverIdle -mp 40 -auto -auto-interval 2s -auto-hyst 3
```

---

### 6) Network mode (download)

```bash
./NeverIdle -network-mode download -n-min 5m -n-max 10m
```

---

### 7) Maintenance scheduler (randomized)

```bash
./NeverIdle -maintenance
```

Optional heartbeat:

```bash
./NeverIdle -maintenance -heartbeat-url "https://example.com/heartbeat"
```

---

### 8) “Prestige Waster” full stack

```bash
./NeverIdle \
  -baseline \
  -smart \
  -n 6h -t 4 \
  -d 1024 -di 1h -path /var/tmp \
  -p 666
```

**Effect:** baseline + smart control + disk/network + lowest priority.

---

## Docker (Build, Run, Compose)

### Build Image

```bash
git clone https://github.com/merisssas/nv.git
cd nv
docker build -t neveridle:latest .
```

### Run Container

> **Note:** The image is **idle by default** to prevent accidental load. Pass `-smart` (CPU + memory + light network) or other flags as needed.

```bash
docker run -d --name neveridle \
  --restart unless-stopped \
  neveridle:latest -smart
```

### Full Examples (Manual → Docker → Compose)

#### Manual (direct binary)

```bash
# Smart (CPU + memory)
./NeverIdle -smart

# Baseline (web + CPU + memory touch + network faker)
./NeverIdle -baseline

# Custom manual (CPU + mem + network + disk)
./NeverIdle -cp 25 -c 1s -mp 40 -n 6h -t 4 -d 1024 -di 1h -path /var/tmp
```

#### Docker run (smart + disk I/O)

```bash
docker run -d --name neveridle \
  --restart unless-stopped \
  -v /var/tmp:/var/tmp \
  neveridle:latest -smart -d 1024 -di 1h -path /var/tmp
```

### Compose

```yaml
services:
  neveridle:
    image: ghcr.io/merisssas/nv:latest
    container_name: neveridle
    restart: unless-stopped
    command: ["-smart", "-d", "1024", "-di", "1h", "-path", "/var/tmp"]
    volumes:
      - /var/tmp:/var/tmp
```

---

## Best Practices

- Use `-smart` for stable, adaptive behavior on most VPS instances.
- Use `-auto` when the VPS is running other workloads.
- Route disk writes to safe volumes (e.g. `/var/tmp`).
- If using TLS, ensure ports 80/443 are open.
- Run inside `tmux`/`screen` to keep the process alive.

---

## Troubleshooting

- **Program exits immediately:** no workers enabled. Use `-smart`, `-baseline`, or at least one of `-cp`/`-m`/`-mp`/`-n`/`-d`.
- **TLS not working:** domain not resolving or ports 80/443 closed.
- **Speedtest failing:** check outbound access to Ookla Speedtest.

---

## Security & Notes

- NeverIdle is designed for controlled resource usage. Always match settings to your provider policies and workload tolerance.
- Do not run it on environments where synthetic resource consumption violates terms of service.
- Use lock files to prevent multiple concurrent instances.
