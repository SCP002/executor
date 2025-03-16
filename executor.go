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

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

// CmdOptions respresents options to create a process.
type CmdOptions struct {
	Command string   // Command to run
	Args    []string // Command arguments
	Dir     string   // Working directory
}

// StartOptions respresents options to start a process.
type StartOptions struct {
	Print      bool                          // Print output to console?
	Capture    bool                          // Build buffer and capture output into Result.Output?
	Wait       bool                          // Wait for program to finish?
	Encoding   *charmap.Charmap              // Endoding.
	NewConsole bool                          // Spawn new console window on Windows?
	Hide       bool                          // Try to hide process window on Windows?
	OnChar     func(c string, p *os.Process) // Callback for each character from process Stdout and Stderr
	OnLine     func(l string, p *os.Process) // Callback for each line from process Stdout and Stderr
}

// Result respresents process run result.
type Result struct {
	DoneOk   bool   // Process exited successfully?
	StartOk  bool   // Process started successfully?
	ExitCode int    // Exit code
	Output   string // Output of Stdout and Stderr
}

// Command respresents command to launch.
type Command struct {
	cmd           *exec.Cmd
	stdoutReader1 *io.PipeReader
	stdoutWriter2 *io.PipeWriter
}

// TODO: https://stackoverflow.com/questions/24677285/how-to-have-multiple-consumer-from-one-io-reader
// TODO: https://stackoverflow.com/questions/10781516/how-to-pipe-several-commands-in-go
// TODO: https://stackoverflow.com/questions/69954944/capture-stdout-from-exec-command-line-by-line-and-also-pipe-to-os-stdout

// NewCommand returns new command with context `ctx` and options `opts`.
func NewCommand(ctx context.Context, opts CmdOptions) *Command {
	cmd := exec.CommandContext(ctx, opts.Command, opts.Args...)
	cmd.Dir = opts.Dir
	cmd.Stdin = os.Stdin // Fix "ERROR: Input redirection is not supported, exiting the process immediately" on Windows

	sigIntCh := make(chan os.Signal, 1)
	signal.Notify(sigIntCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigIntCh
		_ = cmd.Process.Kill()
	}()

	stdoutReader1, stdoutWriter1 := io.Pipe()
	cmd.Stdout = stdoutWriter1

	return &Command{cmd: cmd, stdoutReader1: stdoutReader1}
}

// PipeStdoutTo pipes Stdout to StdIn of `to`.
func (c *Command) PipeStdoutTo(to *Command) {
	stdoutReader1, stdoutWriter1 := io.Pipe()
	stdoutReader2, stdoutWriter2 := io.Pipe()
	c.cmd.Stdout = io.MultiWriter(stdoutWriter1, stdoutWriter2)

	c.stdoutReader1 = stdoutReader1
	to.stdoutWriter2 = stdoutWriter2
	to.cmd.Stdin = stdoutReader2

	// go func() {
	// 	t := time.After(time.Second * 5)
	// 	<-t
	// 	stdoutWriter2.Close()
	// }()
}

// Start starts a process with options `opts`.
func (c *Command) Start(opts StartOptions) (Result, error) {
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
		c.cmd.Stderr = c.cmd.Stdout // Redirect Stderr to Stdout. Must appear after creating a pipe.

		scan := func(reader io.Reader) {
			defer func() {
				scanDoneCh <- struct{}{}
			}()
			if opts.Encoding != nil {
				reader = transform.NewReader(reader, opts.Encoding.NewDecoder())
			}
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
		}
		go scan(c.stdoutReader1)
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
		c.stdoutReader1.Close()
		<-scanDoneCh
	}
	if c.cmd.ProcessState != nil {
		res.DoneOk = c.cmd.ProcessState.Success()
		res.ExitCode = c.cmd.ProcessState.ExitCode()
	}
	res.Output = outSb.String()

	return res, nil
}
