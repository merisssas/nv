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

## Features

- CPU load targeting with optional auto pause/resume.
- Memory allocation by absolute GiB or by target usage percentage.
- Periodic network speed tests to generate bandwidth usage.
- Baseline profile: dummy web server + CPU bump pattern + memory touch + lightweight network faker.
- Disk write bursts with configurable size and interval.
- Graceful shutdown on SIGINT/SIGTERM.
- Optional process priority tuning.

## Requirements

- Linux, macOS, or Windows (Linux recommended).
- Go 1.20+ if building from source.
- For network waste, the host must be able to reach the Ookla Speed Test service.

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
    image: neveridle:latest
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
    image: neveridle:latest
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

- **Burst & Sleep pattern**: CPU usage bursts around **30–40% for a few minutes**, then rests near **0% for a few minutes** to mimic real web traffic.
- `-cp` **Legacy CPU target percent** (used by auto pause/resume thresholds). The burst pattern no longer holds a strict per-second target.
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

### Disk waste

- `-d` **Disk write size in MiB per interval** (e.g., `-d 1024`).
- `-di` **Disk write interval** (e.g., `30m`). Defaults to `30m` when omitted.
- `-path` **Directory for disk writes** (default: current directory).

### Priority

- `-p` **Process priority**. `0` uses normal priority. `19` is lowest on UNIX-like systems. `666` enables background mode (worst priority).

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
