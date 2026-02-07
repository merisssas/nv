# NeverIdle

**Repository:** https://github.com/merisssas/nv

NeverIdle is a lightweight utility that keeps an instance busy by wasting CPU, memory, disk I/O, and/or bandwidth on a schedule. It is intended for environments where idle resources may be reclaimed.

## Advantages

- Combines CPU, memory, disk I/O, and bandwidth waste in one binary for centralized control.
- Can mimic more “realistic” activity through the baseline profile (dummy web server, CPU bump, memory touch, and network faker).
- Automatic CPU and memory control with interval + hysteresis to avoid oscillation.
- Cross-platform support (Linux/macOS/Windows) and runs as a single binary.
- Process priority options to keep workload in the background without disrupting main services.
- Disk write bursts to keep storage activity on a schedule.
- Safe, graceful shutdown when the process stops.
- Maintenance scheduler for randomized compute/network/memory activity plus optional heartbeat pings.
- Built-in lock file to prevent accidental multiple instances on the same host.
- Optional TLS web server with automatic certificates for baseline mode.

## Features

- CPU load targeting with optional auto pause/resume.
- Memory allocation by absolute GiB or by target usage percentage.
- Periodic network speed tests to generate bandwidth usage.
- Baseline profile: dummy web server + CPU bump pattern + memory touch + lightweight network faker.
- Disk write bursts with configurable size and interval.
- Graceful shutdown on SIGINT/SIGTERM.
- Optional process priority tuning.
- Maintenance scheduler with randomized compute/network/memory bursts.
- Optional heartbeat webhook (max once per hour).
- Lock file to prevent duplicate runs.
- Optional TLS (ACME/Let’s Encrypt) for the baseline web server.

## Requirements

- Linux, macOS, or Windows (Linux recommended).
- Go 1.20+ if building from source.
- For network waste, the host must be able to reach the Ookla Speed Test service.
- For baseline TLS, ports 80/443 must be reachable and the domain must resolve to the host.

## Installation

### Option A: Download a release binary

1. Open the Releases page on GitHub: https://github.com/merisssas/nv/releases
2. Download the correct binary for your architecture (amd64 or arm64).
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

1. Download the `Dockerfile`:

```bash
wget https://raw.githubusercontent.com/merisssas/nv/master/Dockerfile
```

2. Build the image:

```bash
# arm machines
sudo docker build -t neveridle:latest .
# amd machines specify ARCH=amd64
sudo docker build --build-arg ARCH=amd64 -t neveridle:latest .
```

3. Run the container:

```bash
sudo docker run -d --name neveridle neveridle:latest -cp 20 -c 1s -m 2 -n 4h
```

## Docker Tutorial (Build, Run, and Compose)

### 1) Build the image yourself

```bash
git clone https://github.com/merisssas/nv.git
cd nv

# ARM (default)
docker build -t neveridle:latest .

# AMD64
docker build --build-arg ARCH=amd64 -t neveridle:latest .
```

### 2) Run the container (docker run)

Simple example:

```bash
docker run -d --name neveridle \
  --restart unless-stopped \
  neveridle:latest -cp 20 -c 1s -m 2 -n 4h
```

Baseline + dummy web server example:

```bash
docker run -d --name neveridle \
  --restart unless-stopped \
  -p 8080:8080 \
  neveridle:latest -baseline -web-addr 0.0.0.0 -web-port 8080
```

### 3) Docker Compose

Create a `docker-compose.yml` like this:

```yaml
services:
  neveridle:
    image: ghcr.io/merisssas/nv:latest
    container_name: neveridle
    restart: unless-stopped
    command: ["-cp", "20", "-c", "1s", "-m", "2", "-n", "4h"]
```

Run:

```bash
docker compose up -d
```

Compose example with baseline + dummy web server:

```yaml
services:
  neveridle:
    image: ghcr.io/merisssas/nv:latest
    container_name: neveridle
    restart: unless-stopped
    ports:
      - "8080:8080"
    command: ["-baseline", "-web-addr", "0.0.0.0", "-web-port", "8080"]
```

> Note: network waste uses a speed test, so the container needs internet access.

## Usage

Start a `screen` or `tmux` session on the server and run the program.

```bash
./NeverIdle -cp 20 -c 1s -m 2 -n 4h
```

When the program starts, it executes each configured worker once so you can verify the effect immediately.

## Command-line flags

> **Note**: Percent values are passed as whole numbers (e.g., `20` for 20%).

### CPU waste

- **Burst & Sleep pattern**: CPU usage bursts around **30–40% for a few minutes**, then rests near **~1% for a few minutes** to mimic real traffic patterns.
- `-cp` **Target CPU percent** for the managed CPU worker. The worker uses a burst/rest pattern rather than a strict per-second target.
- `-c` **Control cycle interval** (e.g., `1s`, `500ms`). If omitted, defaults to `1s`. When `-cp` is not provided, the default CPU target is 35%.
- `-auto` **Auto pause/resume CPU waste** based on system usage (use with `-cp`).
- `-auto-interval` **Auto check interval** (default `2s`).
- `-auto-hyst` **Hysteresis percent** (default `2`) to reduce flapping.

### Memory waste

- `-m` **Memory waste in GiB** (e.g., `-m 6`).
- `-mp` **Target memory usage percent** (e.g., `-mp 60`). This enables automatic memory allocation/release around the target.
- `-auto-interval` and `-auto-hyst` are also used by the memory auto controller.

> `-m` and `-mp` cannot be used together.

### Network waste

- `-n` **Interval for speed tests** (e.g., `45m`, `2h`).
- `-t` **Concurrent connections** for the speed test (default `4`).

### Baseline profile

- `-baseline` **Enable baseline workload**: dummy web server, CPU bump pattern, memory touch, and network faker.
- `-web-addr` **Bind address** for the baseline web server (default `0.0.0.0`).
- `-web-port` **Port** for the baseline web server (default `8080`).
- `-web-tls` **Enable TLS** using automatic certificates (ACME). Requires `-web-domain`.
- `-web-domain` **Domain for TLS certificate** (required when `-web-tls`).

Baseline defaults (when you only pass `-baseline`):
- CPU bump pattern: **40–70% for 90–180s**, then cooldown/idle below 5%.
- Memory touch: **300–800 MiB every 5–15 minutes**.
- Network faker: **2–12 minutes** between activity, **3–7 targets** per cycle.

### Disk waste

- `-d` **Disk write size in MiB per interval** (e.g., `-d 1024`).
- `-di` **Disk write interval** (e.g., `30m`). Defaults to `30m` when omitted.
- `-path` **Directory for disk writes** (default: current directory).

### Priority

- `-p` **Process priority**. `0` uses normal priority. `19` is lowest on UNIX-like systems. `666` enables background mode (worst priority).

### Maintenance scheduler

- `-maintenance` **Enable maintenance scheduler** for dynamic compute/network/memory patterns.
- `-heartbeat-url` **Send a heartbeat webhook** at a randomized interval (max once per hour).

### Safety & control

- `-lock-file` **Lock file path** to prevent multiple instances (default: `/tmp/neveridle.lock`).

## Examples

### Basic CPU + memory + network

```bash
./NeverIdle -cp 20 -c 1s -m 2 -n 4h
```

### Baseline workload (web server + periodic CPU/memory/network)

```bash
./NeverIdle -baseline
```

### CPU auto pause/resume

```bash
./NeverIdle -cp 25 -c 1s -auto -auto-interval 2s -auto-hyst 3
```

### Memory target percentage (auto)

```bash
./NeverIdle -mp 60 -auto-interval 2s -auto-hyst 5
```

### Disk write bursts

```bash
./NeverIdle -d 1024 -di 30m -path /var/tmp
```

### Lowest priority background mode

```bash
./NeverIdle -cp 20 -c 1s -p 666
```

## Stopping NeverIdle

Press `Ctrl+C` in the terminal, or stop the container with:

```bash
sudo docker stop neveridle
```

## Full Tutorial (All Capabilities)

This section walks through **every capability** and how to combine them safely.

### 1) Quick start: keep a VM warm with CPU + memory + network

```bash
./NeverIdle -cp 20 -c 1s -m 2 -n 4h
```

What this does:
- Keeps CPU active with a burst/rest pattern centered on ~20% utilization.
- Allocates ~2 GiB of memory.
- Runs a network speed test every 4 hours.

### 2) Baseline mode (realistic background activity)

Baseline is the easiest “set and forget” mode:

```bash
./NeverIdle -baseline
```

What it includes:
- **Web server** on `0.0.0.0:8080` with `/` and `/healthz`.
- **CPU bump pattern**: 40–70% for 90–180 seconds, then low-usage cooldown/idle.
- **Memory touch**: 300–800 MiB every 5–15 minutes.
- **Network faker**: DNS lookups and lightweight HTTP fetches.

### 3) Enable TLS for the baseline web server

> Requires a public domain pointing to the host, and open ports 80/443.

```bash
./NeverIdle -baseline -web-tls -web-domain your-domain.example
```

Notes:
- TLS certificates are stored under `./certs`.
- The ACME HTTP challenge listens on port 80.

### 4) Maintenance scheduler (dynamic randomized activity)

Maintenance mode is a fully randomized pattern meant to look organic:

```bash
./NeverIdle -maintenance
```

What it does:
- Runs periodic compute bursts with random duration and CPU ratio.
- Generates random network requests to public targets.
- On ARM64, performs intermittent memory bursts.

#### Add heartbeat ping (optional)

```bash
./NeverIdle -maintenance -heartbeat-url "https://example.com/heartbeat"
```

The heartbeat sends at a randomized interval (max once per hour).

### 5) CPU control options

#### Fixed CPU pattern

```bash
./NeverIdle -cp 25 -c 1s
```

#### Auto pause/resume CPU (adaptive)

```bash
./NeverIdle -cp 25 -c 1s -auto -auto-interval 2s -auto-hyst 3
```

Use this if you want the worker to back off when your system is already busy.

### 6) Memory control options

#### Fixed memory allocation

```bash
./NeverIdle -m 6
```

#### Target memory percentage (auto)

```bash
./NeverIdle -mp 60 -auto-interval 2s -auto-hyst 5
```

> `-m` and `-mp` cannot be used together.

### 7) Network usage options

#### Speed test interval

```bash
./NeverIdle -n 2h -t 6
```

This runs an Ookla speed test every 2 hours using 6 connections.

### 8) Disk write bursts

```bash
./NeverIdle -d 1024 -di 30m -path /var/tmp
```

This writes 1 GiB every 30 minutes to the target path.

### 9) Priority control (run in background)

```bash
./NeverIdle -cp 20 -p 666
```

This sets the process to the lowest priority (background mode).

### 10) Lock file (avoid duplicate runs)

```bash
./NeverIdle -lock-file /tmp/neveridle.lock -cp 20
```

If another instance is already running with the same lock file, it will exit safely.

### 11) Combine everything (example profile)

```bash
./NeverIdle \
  -baseline \
  -cp 20 -c 1s \
  -mp 55 \
  -n 6h -t 4 \
  -d 1024 -di 1h -path /var/tmp \
  -p 666
```

This combination:
- Runs baseline background activity and exposes a web status page.
- Targets 20% CPU with burst/rest pattern.
- Maintains memory around 55% utilization.
- Performs periodic network tests.
- Writes disk bursts hourly.
- Stays at lowest priority.

## Troubleshooting

- **No output / exits immediately**: you likely didn’t pass any waste flags. Run with `-baseline` or at least one of `-cp`, `-m`, `-mp`, `-n`, or `-d`.
- **TLS not working**: ensure the domain resolves to the host and ports 80/443 are open.
- **Speed test fails**: check outbound access to Ookla SpeedTest endpoints.
