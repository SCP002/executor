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

// Options respresents options to start process
type Options struct {
	Command    string                        // Command to run
	Args       []string                      // Command arguments
	Print      bool                          // Print output to console?
	Capture    bool                          // Build buffer and capture output into Result.Output?
	Wait       bool                          // Wait for program to finish?
	Timeout    uint                          // Time in seconds allotted for the execution of the process before it get killed
	NewConsole bool                          // Spawn new console window on Windows?
	Hide       bool                          // Try to hide process window on Windows?
	OnChar     func(c string, p *os.Process) // Callback for each character from process StdOut and StdErr
	OnLine     func(l string, p *os.Process) // Callback for each line from process StdOut and StdErr
}

// Result respresents process run result
type Result struct {
	ExitCode int
	Output   string
}

// Start starts a process
func Start(opts Options) Result {
	res := Result{
		ExitCode: -1,
	}

	var outSb strings.Builder
	var err error

	// Create context for command (empty or with timeout)
	ctx := context.Background()
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
		defer cancel()
	}

	// Create command
	cmd := exec.CommandContext(ctx, opts.Command, opts.Args...)

	// Fix "ERROR: Input redirection is not supported, exiting the process immediately" on Windows
	cmd.Stdin = os.Stdin

	if opts.NewConsole || opts.Hide {
		setCmdAttr(cmd, opts.NewConsole, opts.Hide)

		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
	} else { // Can capture output
		stdoutReader, err := cmd.StdoutPipe()

		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return res
		}

		// Redirect StdErr to StdOut
		cmd.Stderr = cmd.Stdout

		stdoutScanner := bufio.NewScanner(stdoutReader)
		stdoutScanner.Split(bufio.ScanRunes)
		var lineSb strings.Builder

		// Scan output
		go func() {
			for stdoutScanner.Scan() {
				char := stdoutScanner.Text()
				if opts.Print {
					fmt.Print(char)
				}
				if opts.Capture {
					outSb.WriteString(char)
				}
				// Char callback
				if opts.OnChar != nil {
					opts.OnChar(char, cmd.Process)
				}
				// Build the line
				if opts.OnLine != nil {
					if char != "\n" && char != "\r" {
						lineSb.WriteString(char)
					} else {
						// Line callback
						opts.OnLine(lineSb.String(), cmd.Process)
						lineSb.Reset()
					}
				}
			}
		}()
	}

	// Start the command
	err = cmd.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return res
	}

	// Wait for the command to finish execution
	if opts.Wait {
		err = cmd.Wait()
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n%v\n", err)
			if ctx.Err() != nil {
				fmt.Fprintln(os.Stderr, ctx.Err())
			}
			return res
		}
	}

	// Build and return Result
	res.ExitCode = cmd.ProcessState.ExitCode()
	res.Output = outSb.String()

	return res
}
