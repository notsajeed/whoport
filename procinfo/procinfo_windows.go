// Package procinfo answers "what do I know about this PID?" using
// Win32 process APIs directly. Command line is the one exception —
// Windows doesn't expose it through a simple handle-based call the way
// Linux exposes /proc/<pid>/cmdline, so we go through PowerShell/WMI
// for just that one field rather than reading another process's PEB
// (Process Environment Block), which is a much deeper rabbit hole than
// v1 needs.
package procinfo

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

type ProcessInfo struct {
	PID          uint32
	Name         string
	Path         string
	CommandLine  string
	StartTime    time.Time
	Exists       bool
	AccessDenied bool // process is alive but we lack rights to inspect it
}

// Get gathers everything we can about a PID. Exists is false when the
// process has already exited (common — ports get filtered a moment
// after a process dies). AccessDenied is true when the process is
// alive but we don't have permission to inspect it — usually means
// running the tool without Administrator rights against a
// system/elevated process. Run elevated to resolve those.
func Get(pid uint32) ProcessInfo {
	info := ProcessInfo{PID: pid}

	handle, err := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false,
		pid,
	)
	if err != nil {
		if err == windows.ERROR_ACCESS_DENIED {
			info.AccessDenied = true
		}
		return info // process exited, or access denied (see AccessDenied)
	}
	defer windows.CloseHandle(handle)
	info.Exists = true

	// Full executable path.
	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err == nil {
		info.Path = windows.UTF16ToString(buf[:size])
		if idx := strings.LastIndexAny(info.Path, `\/`); idx != -1 {
			info.Name = info.Path[idx+1:]
		} else {
			info.Name = info.Path
		}
	}

	// Start time.
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err == nil {
		info.StartTime = time.Unix(0, creation.Nanoseconds())
	}

	info.CommandLine = commandLineViaWMI(pid)

	return info
}

// commandLineViaWMI shells out to PowerShell's CIM cmdlets to fetch the
// full command line for a PID. This is the one part of the tool that
// isn't a direct syscall — pragmatic tradeoff for v1 over reading the
// process's PEB manually.
func commandLineViaWMI(pid uint32) string {
	script := fmt.Sprintf(
		`(Get-CimInstance Win32_Process -Filter "ProcessId=%d").CommandLine`,
		pid,
	)
	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Kill terminates the process. Returns an error if it fails (e.g.
// access denied — protected/system process, or not running as admin).
func Kill(pid uint32) error {
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return fmt.Errorf("cannot open process %d for termination: %w", pid, err)
	}
	defer windows.CloseHandle(handle)

	if err := windows.TerminateProcess(handle, 1); err != nil {
		return fmt.Errorf("failed to terminate process %d: %w", pid, err)
	}
	return nil
}