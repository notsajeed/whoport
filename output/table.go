package output

import (
	"fmt"
	"time"

	"whoport/porttable"
	"whoport/procinfo"
)

// PrintDetail prints the full breakdown for a single port match — the
// output shown by `whoport 3000`.
func PrintDetail(entry porttable.PortEntry, proc procinfo.ProcessInfo) {
	fmt.Printf("\nPort %d (TCP, %s)\n", entry.LocalPort, entry.State)

	if !proc.Exists {
		if proc.AccessDenied {
			fmt.Printf("  PID:      %d (access denied — try running as Administrator)\n", entry.PID)
		} else {
			fmt.Printf("  PID:      %d (process has already exited)\n", entry.PID)
		}
		return
	}

	fmt.Printf("  PID:      %d\n", proc.PID)
	fmt.Printf("  Process:  %s\n", proc.Name)
	fmt.Printf("  Path:     %s\n", proc.Path)
	if proc.CommandLine != "" {
		fmt.Printf("  Command:  %s\n", proc.CommandLine)
	}
	if !proc.StartTime.IsZero() {
		fmt.Printf("  Started:  %s ago\n", formatDuration(time.Since(proc.StartTime)))
	}
	fmt.Println()
}

// PrintTable prints a compact multi-row table — used by `whoport --all`.
func PrintTable(rows []struct {
	Entry porttable.PortEntry
	Proc  procinfo.ProcessInfo
}) {
	fmt.Printf("%-8s %-10s %-8s %-25s\n", "PORT", "STATE", "PID", "PROCESS")
	fmt.Println("--------------------------------------------------")
	for _, r := range rows {
		name := r.Proc.Name
		if name == "" {
			switch {
			case r.Proc.AccessDenied:
				name = "(access denied — run as admin)"
			case !r.Proc.Exists:
				name = "(exited)"
			default:
				name = "(unknown)"
			}
		}
		fmt.Printf("%-8d %-10s %-8d %-25s\n", r.Entry.LocalPort, r.Entry.State, r.Entry.PID, name)
	}
}

func formatDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
	}
}