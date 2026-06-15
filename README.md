
<p align="center">
  <img src="https://github.com/popmanpop27/zapretstat/blob/main/screenshot.png?raw=true" alt="zapretstat screenshot" width="100%">
</p>

<h1 align="center">zapretstat</h1>

<p align="center">
  Lightweight terminal monitor for internet availability, latency spikes and outages.
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go" />
  <img src="https://img.shields.io/badge/platform-linux%20%7C%20macOS%20%7C%20windows-lightgrey" />
  <img src="https://img.shields.io/badge/TUI-gotui%20v5-blue" />
</p>

---

## About

**zapretstat** is a lightweight TUI monitor for visualizing internet connectivity and latency history.

It reads telemetry logs (`jsonl`) produced by `zapretstatd` and displays:

- real-time latency graph
- internet outages
- recovery events
- unreachable resource events
- latency spikes detection
- uptime percentage
- event timeline

The interface automatically refreshes and works directly in the terminal.

It contains 2 components: internet checker(runs in systemd on linux), is gains logs about internet activity and stores it in file, and the client which draws graph in terminal.

---

## Features

### Live latency graph

Visualize network latency over time.

  The latency spikes visualized with !
  Time without internet connection visualized with ⚑⚑⚑⚑

## Installation

Clone repository:

```bash
git clone https://github.com/yourname/zapretstat.git
cd zapretstat
```

Build:

```bash
go build ./...
```

Or run directly:

```bash
go run ./zapretstatd
```

---

## Log file location

`zapretstat` automatically finds the telemetry file depending on OS.

### Linux

```text
~/.config/zapretstat/zstats.jsonl
```

or

```text
$XDG_CONFIG_HOME/zapretstat/zstats.jsonl
```

### macOS

```text
~/Library/Application Support/zapretstat/zstats.jsonl
```

### Windows

```text
%APPDATA%\zapretstat\zstats.jsonl
```

---

## Controls

| Key       | Action        |
| --------- | ------------- |
| `q`       | quit          |
| `Ctrl+C`  | quit          |
| `r`       | refresh       |

---

## JSONL format

Example telemetry:

```json
{
  "ts": "2026-06-15T12:00:00Z",
  "type": "sample",
  "latency_ms": 43,
  "internet": true
}
```

Example event:

```json
{
  "ts": "2026-06-15T12:01:00Z",
  "type": "event",
  "event": "internet_down",
  "message": "connection lost"
}
```

---

## Why?

Many network monitors are:

* too heavy
* GUI-only
* focused on throughput instead of stability

`zapretstat` focuses on a single thing:

> quickly seeing **whether the internet is actually stable**.

Especially useful when debugging flaky ISP routing, VPNs, DPI bypass tools or intermittent connectivity issues.

---

## Stack

* Go
* gotui v5
* JSONL telemetry
* terminal UI

---

## License

MIT

```
```
