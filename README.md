# whoport

A CLI tool for Windows that tells you exactly what's listening on a port — and lets you kill it — without the `netstat -ano | findstr` → `tasklist | findstr` dance.

```
> whoport 8123

Port 8123 (TCP, LISTENING)
  PID:      7524
  Process:  python.exe
  Path:     C:\Users\modi\AppData\Local\Python\bin\python.exe
  Command:  "C:\Users\modi\AppData\Local\Python\bin\python.exe" -m http.server 8123
  Started:  16s ago

Kill python.exe (PID 7524)? [y/N]:
```

## Why

Every dev hits `Error: listen EADDRINUSE: address already in use :::3000` eventually. The normal Windows workflow to fix it is four separate steps and two different ugly commands. `whoport` does it in one, and shows you enough context (full path, command line, uptime) to actually be sure you're killing the right thing before you kill it.

Unlike tools that shell out to `netstat`/`taskkill` and parse text output, `whoport` calls the Windows IP Helper API (`GetExtendedTcpTable`) and process APIs directly — the same underlying mechanism `netstat` itself uses, just without the middleman.

## Install

**Prebuilt binary** (no Go required): grab `whoport.exe` from [Releases](../../releases) and put it somewhere on your `PATH`.

**Build from source** (requires [Go](https://go.dev) 1.22+):

```powershell
git clone https://github.com/notsajeed/whoport.git
cd whoport
go build -o whoport.exe .
```

## Usage

**Check what's on a port:**

```powershell
whoport 3000
```

Shows PID, process name, full path, command line, and how long it's been running. Prompts before killing.

**List everything currently listening:**

```powershell
whoport --all
```

**Kill without the confirmation prompt** (for scripting):

```powershell
whoport 3000 --kill
```

## Notes

- **Run as Administrator** to see full process details for system/elevated processes — without it, some rows will show `access denied` instead of a process name. This is a Windows permissions thing, not a bug.
- PID `4` is always the Windows kernel's `System` process — it'll never resolve to a name, even elevated. Expected.
- `whoport` only reads the TCP table and (optionally) terminates the specific process you confirm. It doesn't touch the registry, network config, or anything else on your system.

## How it works

- `porttable/` — calls `GetExtendedTcpTable` (iphlpapi.dll) directly to enumerate the TCP table; no `netstat` parsing
- `procinfo/` — resolves a PID to process name/path/start-time via `OpenProcess`/`QueryFullProcessImageName`/`GetProcessTimes`; command line comes from a WMI query via PowerShell (the one part of the tool that isn't a raw syscall)
- `output/` — formatting only

## Roadmap

- [ ] UDP support
- [ ] `--watch <port>` — alert when a port opens/closes
- [ ] `--dry-run` — preview a kill without executing it
- [ ] Cross-platform (Linux/macOS via `/proc/net/tcp`)

## License

MIT
