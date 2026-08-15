
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"

	"whoport/output"
	"whoport/porttable"
	"whoport/procinfo"
)

func main() {
	all := flag.Bool("all", false, "list every listening TCP port")
	kill := flag.Bool("kill", false, "kill the process without confirmation prompt")
	flag.Parse()

	entries, err := porttable.GetTCPTable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading TCP table: %v\n", err)
		os.Exit(1)
	}

	if *all {
		runAll(entries)
		return
	}

	args := flag.Args()
	if len(args) != 1 {
		fmt.Println("usage: whoport <port>       show what's listening on a port")
		fmt.Println("       whoport --all         list all listening ports")
		fmt.Println("       whoport <port> --kill kill without confirmation")
		os.Exit(1)
	}

	port, err := strconv.Atoi(args[0])
	if err != nil || port < 1 || port > 65535 {
		fmt.Fprintf(os.Stderr, "invalid port: %s\n", args[0])
		os.Exit(1)
	}

	runSingle(entries, uint16(port), *kill)
}

func runSingle(entries []porttable.PortEntry, port uint16, killFlag bool) {
	matches := porttable.FilterByPort(entries, port)
	if len(matches) == 0 {
		fmt.Printf("Nothing is listening on port %d.\n", port)
		return
	}

	for _, entry := range matches {
		proc := procinfo.Get(entry.PID)
		output.PrintDetail(entry, proc)

		if !proc.Exists {
			continue
		}

		if killFlag {
			doKill(entry.PID)
			continue
		}

		if confirm(fmt.Sprintf("Kill %s (PID %d)?", proc.Name, proc.PID)) {
			doKill(entry.PID)
		}
	}
}

func runAll(entries []porttable.PortEntry) {
	listening := porttable.FilterListening(entries)
	rows := make([]struct {
		Entry porttable.PortEntry
		Proc  procinfo.ProcessInfo
	}, 0, len(listening))

	for _, e := range listening {
		rows = append(rows, struct {
			Entry porttable.PortEntry
			Proc  procinfo.ProcessInfo
		}{Entry: e, Proc: procinfo.Get(e.PID)})
	}

	output.PrintTable(rows)
}

func doKill(pid uint32) {
	if err := procinfo.Kill(pid); err != nil {
		fmt.Fprintf(os.Stderr, "  failed to kill: %v\n", err)
		return
	}
	fmt.Printf("  killed PID %d.\n", pid)
}

func confirm(prompt string) bool {
	fmt.Printf("%s [y/N]: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return line == "y\n" || line == "Y\n" || line == "y\r\n" || line == "Y\r\n"
}