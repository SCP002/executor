package executor

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Options respresents options to start process.
type Options struct {
	Command    string                        // Command to run
	Args       []string                      // Command arguments
	Print      bool                          // Print output to console?
	Capture    bool                          // Build buffer and capture output into Result.Output?
	Wait       bool                          // Wait for program to finish?
	Timeout    uint                          // Time in seconds allotted for the execution of the process before it get killed
	Dir        string                        // Working directory
	NewConsole bool                          // Spawn new console window on Windows?
	Hide       bool                          // Try to hide process window on Windows?
	OnChar     func(c string, p *os.Process) // Callback for each character from process StdOut and StdErr
	OnLine     func(l string, p *os.Process) // Callback for each line from process StdOut and StdErr
}

// Result respresents process run result.
type Result struct {
	DoneOk   bool   // Process exited successfully?
	StartOk  bool   // Process started successfully?
	ExitCode int    // Exit code
	Output   string // Output of StdOut and StdErr
}

// Start starts a process with options `opts`.
func Start(opts Options) (Result, error) {
	res := Result{
		ExitCode: -1,
	}

	ctx := context.Background()
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, opts.Command, opts.Args...)
	cmd.Dir = opts.Dir
	cmd.Stdin = os.Stdin // Fix "ERROR: Input redirection is not supported, exiting the process immediately" on Windows

	var outSb strings.Builder

	if opts.NewConsole || opts.Hide {
		setCmdAttr(cmd, opts.NewConsole, opts.Hide)

		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
	} else { // Can capture output
		stdoutReader, err := cmd.StdoutPipe()
		if err != nil {
			return res, fmt.Errorf("Create stdout pipe: %w", err)
		}
		cmd.Stderr = cmd.Stdout // Redirect StdErr to StdOut
		stdoutScanner := bufio.NewScanner(stdoutReader)
		stdoutScanner.Split(bufio.ScanRunes)

		scan := func(scanner *bufio.Scanner) {
			var lineSb strings.Builder
			for scanner.Scan() {
				char := scanner.Text()
				if opts.Print {
					fmt.Print(char)
				}
				if opts.Capture {
					outSb.WriteString(char)
				}
				if opts.OnChar != nil {
					opts.OnChar(char, cmd.Process)
				}
				if opts.OnLine != nil {
					if char == "\n" || char == "\r" {
						opts.OnLine(lineSb.String(), cmd.Process)
						lineSb.Reset()
					} else {
						lineSb.WriteString(char)
					}
				}
			}
		}
		go scan(stdoutScanner)
	}

	err := cmd.Start()
	if err != nil {
		return res, fmt.Errorf("Start process: %w", err)
	}
	res.StartOk = true

	if opts.Wait {
		if err = cmd.Wait(); err != nil {
			return res, fmt.Errorf("Wait for process: %w", err)
		}
	}
	if cmd.ProcessState != nil {
		res.DoneOk = cmd.ProcessState.Success()
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	res.Output = outSb.String()

	return res, nil
}
