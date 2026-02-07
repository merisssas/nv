# NeverIdle

**Repository:** https://github.com/merisssas/nv

NeverIdle is a lightweight utility that keeps an instance busy by wasting CPU, memory, disk I/O, and/or bandwidth on a schedule. It is intended for environments where idle resources may be reclaimed.

## Features

- CPU load targeting with optional auto pause/resume.
- Memory allocation by absolute GiB or by target usage percentage.
- Periodic network speed tests to generate bandwidth usage.
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
