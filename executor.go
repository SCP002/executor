package executor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// CmdOptions respresents options to create a process.
type CmdOptions struct {
	Command    string                        // Command to run
	Args       []string                      // Command arguments
	Timeout    uint                          // Time in seconds allotted for the execution of the process before it gets killed
	Dir        string                        // Working directory
}

// StartOptions respresents options to start a process.
type StartOptions struct {
	Print      bool                          // Print output to console?
	Capture    bool                          // Build buffer and capture output into Result.Output?
	Wait       bool                          // Wait for program to finish?
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

// Command respresents command to launch.
type Command struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

// NewCommand returns new command with options `opts`.
func NewCommand(opts CmdOptions) *Command {
	ctx := context.Background()
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
	}

	cmd := exec.CommandContext(ctx, opts.Command, opts.Args...)
	cmd.Dir = opts.Dir
	cmd.Stdin = os.Stdin // Fix "ERROR: Input redirection is not supported, exiting the process immediately" on Windows

	sigIntCh := make(chan os.Signal, 1)
	signal.Notify(sigIntCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigIntCh
		_ = cmd.Process.Kill()
	}()

	return &Command{cmd: cmd, cancel: cancel}
}

// Start starts a process with options `opts`.
func (c Command) Start(opts StartOptions) (Result, error) {
	res := Result{
		ExitCode: -1,
	}

	var outSb strings.Builder
	scanDoneCh := make(chan struct{}, 1)

	if opts.NewConsole || opts.Hide {
		setCmdAttr(c.cmd, opts.NewConsole, opts.Hide)

		c.cmd.Stderr = os.Stderr
		c.cmd.Stdout = os.Stdout
	} else { // Can capture output
		stdoutReader, err := c.cmd.StdoutPipe()
		if err != nil {
			return res, fmt.Errorf("Create stdout pipe: %w", err)
		}
		c.cmd.Stderr = c.cmd.Stdout // Redirect StdErr to StdOut. Must appear after creating a pipe.

		scan := func(reader io.ReadCloser) {
			var lineSb strings.Builder
			scanner := bufio.NewScanner(reader)
			scanner.Split(bufio.ScanRunes)
			for scanner.Scan() {
				char := scanner.Text()
				if opts.Print {
					fmt.Print(char)
				}
				if opts.Capture {
					outSb.WriteString(char)
				}
				if opts.OnChar != nil {
					opts.OnChar(char, c.cmd.Process)
				}
				if opts.OnLine != nil {
					if char == "\n" || char == "\r" {
						opts.OnLine(lineSb.String(), c.cmd.Process)
						lineSb.Reset()
					} else {
						lineSb.WriteString(char)
					}
				}
			}
			scanDoneCh <- struct{}{}
		}
		go scan(stdoutReader)
	}

	err := c.cmd.Start()
	if err != nil {
		return res, fmt.Errorf("Start process: %w", err)
	}
	res.StartOk = true

	if opts.Wait {
		exitErr := &exec.ExitError{}
		if err = c.cmd.Wait(); err != nil && !errors.As(err, &exitErr) {
			return res, fmt.Errorf("Wait for process: %w", err)
		}
		<-scanDoneCh
		if c.cancel != nil {
			c.cancel()
		}
	}
	if c.cmd.ProcessState != nil {
		res.DoneOk = c.cmd.ProcessState.Success()
		res.ExitCode = c.cmd.ProcessState.ExitCode()
	}
	res.Output = outSb.String()

	return res, nil
}
