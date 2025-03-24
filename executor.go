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
	cmd        *exec.Cmd
	prevCmd    *Command
	pipeReader *io.PipeReader
	pipeWriter *io.PipeWriter
	recvStdout bool
	recvStderr bool
	sendStdout bool
	sendStderr bool
}

// ReadCloser implements io.ReadCloser.
type ReadCloser struct {
	io.Reader
	io.Closer
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
	c.sendStdout = true
	to.recvStdout = true
	to.prevCmd = c
}

// PipeStderrTo pipes Stderr to Stdin of `to`.
func (c *Command) PipeStderrTo(to *Command) {
	c.sendStderr = true
	to.recvStderr = true
	to.prevCmd = c
}

// Start starts a process with options `opts`.
func (c *Command) Start(opts StartOptions) (Result, error) {
	res := Result{
		ExitCode: -1,
	}

	var outSb strings.Builder
	scanDoneCh := make(chan struct{}, 1)

	var stdoutReader io.ReadCloser
	var stdoutWriter io.WriteCloser
	stdoutReader, stdoutWriter = io.Pipe()
	if opts.ScanStdout || c.sendStdout {
		c.cmd.Stdout = stdoutWriter
	}

	var stderrReader io.ReadCloser
	var stderrWriter io.WriteCloser
	stderrReader, stderrWriter = io.Pipe()
	if opts.ScanStderr || c.sendStderr {
		c.cmd.Stderr = stderrWriter
	}

	combinedReader, combinedWriter := io.Pipe()

	if opts.NewConsole || opts.Hide {
		setCmdAttr(c.cmd, opts.NewConsole, opts.Hide)

		c.cmd.Stderr = os.Stderr
		c.cmd.Stdout = os.Stdout
	} else { // Can capture output.
		scan := func(reader io.Reader) {
			defer func() {
				scanDoneCh <- struct{}{}
			}()
			var lineSb strings.Builder
			scanner := bufio.NewScanner(reader)
			scanner.Split(bufio.ScanRunes)
			for scanner.Scan() {
				char := scanner.Text()
				if opts.Capture {
					outSb.WriteString(char)
				}
				if opts.OnChar != nil {
					opts.OnChar(char, c.cmd.Process)
				}
				if opts.OnLine != nil {
					if char == "\n" {
						opts.OnLine(lineSb.String(), c.cmd.Process)
						lineSb.Reset()
					} else {
						lineSb.WriteString(char)
					}
				}
			}
		}

		if opts.ScanStdout && opts.Encoding != nil {
			transformReader := transform.NewReader(stdoutReader, opts.Encoding.NewDecoder())
			stdoutReader = ReadCloser{transformReader, stdoutReader}
		}
		if opts.ScanStdout && opts.Print {
			tee := io.TeeReader(stdoutReader, os.Stdout)
			stdoutReader = ReadCloser{tee, stdoutReader}
		}

		if opts.ScanStderr && opts.Encoding != nil {
			transformReader := transform.NewReader(stderrReader, opts.Encoding.NewDecoder())
			stderrReader = ReadCloser{transformReader, stderrReader}
		}
		if opts.ScanStderr && opts.Print {
			tee := io.TeeReader(stderrReader, os.Stderr)
			stderrReader = ReadCloser{tee, stderrReader}
		}

		if c.sendStdout || c.sendStderr {
			c.pipeReader, c.pipeWriter = io.Pipe()
		}
		if c.sendStdout {
			if opts.ScanStdout {
				c.cmd.Stdout = io.MultiWriter(c.cmd.Stdout, c.pipeWriter)
			} else {
				c.cmd.Stdout = c.pipeWriter
			}
		}
		if c.sendStderr {
			if opts.ScanStderr {
				c.cmd.Stderr = io.MultiWriter(c.cmd.Stderr, c.pipeWriter)
			} else {
				c.cmd.Stderr = c.pipeWriter
			}
		}

		if c.recvStdout || c.recvStderr {
			c.cmd.Stdin = c.prevCmd.pipeReader
		}

		if opts.ScanStdout && opts.ScanStderr {
			go io.Copy(combinedWriter, stdoutReader)
			go io.Copy(combinedWriter, stderrReader)
			go scan(combinedReader)
		} else if opts.ScanStdout {
			go scan(stdoutReader)
		} else if opts.ScanStderr {
			go scan(stderrReader)
		}
	}

	err := c.cmd.Start()
	if err != nil {
		return res, fmt.Errorf("Start process: %w", err)
	}
	res.StartOk = true

	if c.prevCmd != nil {
		if err := c.prevCmd.cmd.Wait(); err != nil {
			return res, fmt.Errorf("Wait for previous process: %w", err)
		}
	}

	if c.prevCmd != nil && c.prevCmd.pipeWriter != nil {
		c.prevCmd.pipeWriter.Close()
	}

	if opts.Wait {
		exitErr := &exec.ExitError{}
		if err = c.cmd.Wait(); err != nil && !errors.As(err, &exitErr) {
			return res, fmt.Errorf("Wait for process: %w", err)
		}
		if stdoutReader != nil {
			stdoutReader.Close()
		}
		if stderrReader != nil {
			stderrReader.Close()
		}
		if c.pipeReader != nil {
			c.pipeReader.Close()
		}
		combinedReader.Close()
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
