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
	Command string   // Command to run.
	Args    []string // Command arguments.
	Dir     string   // Working directory.
}

// StartOptions respresents options to start a process.
type StartOptions struct {
	ScanStdout bool                          // Scan for Stdout (Capture + Print)?
	ScanStderr bool                          // Scan for Stderr (Capture + Print)?
	Print      bool                          // Print output?
	Capture    bool                          // Build buffer and capture output into Result.Output?
	Wait       bool                          // Wait for program to finish?
	Encoding   *charmap.Charmap              // Endoding.
	NewConsole bool                          // Spawn new console window on Windows?
	Hide       bool                          // Try to hide process window on Windows?
	OnChar     func(c string, p *os.Process) // Callback for each character.
	OnLine     func(l string, p *os.Process) // Callback for each line.
}

// Result respresents process run result.
type Result struct {
	DoneOk   bool   // Process exited successfully?
	StartOk  bool   // Process started successfully?
	ExitCode int    // Exit code.
	Output   string // Captured output.
}

// Command respresents command to launch.
type Command struct {
	cmd              *exec.Cmd
	prevCmd          *exec.Cmd
	stdoutScanReader *io.PipeReader
	stderrScanReader *io.PipeReader
	stdoutPipeWriter *io.PipeWriter
	stderrPipeWriter *io.PipeWriter
}

// NewCommand returns new command with context `ctx` and options `opts`.
func NewCommand(ctx context.Context, opts CmdOptions) *Command {
	cmd := exec.CommandContext(ctx, opts.Command, opts.Args...)
	cmd.Dir = opts.Dir
	cmd.Stdin = os.Stdin // Fix "ERROR: Input redirection is not supported, exiting the process immediately" on Windows.

	sigIntCh := make(chan os.Signal, 1)
	signal.Notify(sigIntCh, os.Interrupt, syscall.SIGTERM) // Fix broken console on Ctrl + C.

	return &Command{cmd: cmd}
}

// PipeStdoutTo pipes Stdout to Stdin of `to`.
func (c *Command) PipeStdoutTo(to *Command) {
	if c.stderrScanReader != nil {
		c.cmd.Stdout = c.cmd.Stderr
		return
	}

	stdoutScanReader, stdoutScanWriter := io.Pipe()
	stdoutPipeReader, stdoutPipeWriter := io.Pipe()
	c.cmd.Stdout = io.MultiWriter(stdoutScanWriter, stdoutPipeWriter)

	c.stdoutScanReader = stdoutScanReader
	to.stdoutPipeWriter = stdoutPipeWriter
	to.cmd.Stdin = stdoutPipeReader
	to.prevCmd = c.cmd
}

// PipeStderrTo pipes Stderr to Stdin of `to`.
func (c *Command) PipeStderrTo(to *Command) {
	if c.stdoutScanReader != nil {
		c.cmd.Stderr = c.cmd.Stdout
		return
	}

	stderrScanReader, stderrScanWriter := io.Pipe()
	stderrPipeReader, stderrPipeWriter := io.Pipe()
	c.cmd.Stderr = io.MultiWriter(stderrScanWriter, stderrPipeWriter)

	c.stderrScanReader = stderrScanReader
	to.stderrPipeWriter = stderrPipeWriter
	to.cmd.Stdin = stderrPipeReader
	to.prevCmd = c.cmd
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
	} else { // Can capture output.
		if c.stdoutScanReader == nil && opts.ScanStdout {
			stdoutScanReader, stdoutScanWriter := io.Pipe()
			c.cmd.Stdout = stdoutScanWriter
			c.stdoutScanReader = stdoutScanReader
		}
		if c.stderrScanReader == nil && opts.ScanStderr {
			stderrScanReader, stderrScanWriter := io.Pipe()
			c.cmd.Stderr = stderrScanWriter
			c.stderrScanReader = stderrScanReader
		}
		if opts.ScanStdout && opts.ScanStderr {
			c.cmd.Stderr = c.cmd.Stdout // Redirect Stderr to Stdout. Must appear after creating a pipe.
		}

		scan := func(reader io.Reader, printToStdout bool) {
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
					if printToStdout {
						fmt.Print(char)
					} else {
						fmt.Fprint(os.Stderr, char)
					}
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

		if c.stdoutScanReader != nil {
			go scan(c.stdoutScanReader, true)
		} else if c.stderrScanReader != nil {
			go scan(c.stderrScanReader, false)
		}
	}

	err := c.cmd.Start()
	if err != nil {
		return res, fmt.Errorf("Start process: %w", err)
	}
	res.StartOk = true

	if c.prevCmd != nil {
		if err := c.prevCmd.Wait(); err != nil {
			return res, fmt.Errorf("Wait for previous process: %w", err)
		}
	}
	if c.stdoutPipeWriter != nil {
		c.stdoutPipeWriter.Close()
	}
	if c.stderrPipeWriter != nil {
		c.stderrPipeWriter.Close()
	}

	if opts.Wait {
		exitErr := &exec.ExitError{}
		if err = c.cmd.Wait(); err != nil && !errors.As(err, &exitErr) {
			return res, fmt.Errorf("Wait for process: %w", err)
		}
		if c.stdoutScanReader != nil {
			c.stdoutScanReader.Close()
		}
		if c.stderrScanReader != nil {
			c.stderrScanReader.Close()
		}
		if opts.ScanStderr || opts.ScanStdout {
			<-scanDoneCh
		}
	}

	if c.cmd.ProcessState != nil {
		res.DoneOk = c.cmd.ProcessState.Success()
		res.ExitCode = c.cmd.ProcessState.ExitCode()
	}
	res.Output = outSb.String()

	return res, nil
}
