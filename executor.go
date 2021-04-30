package executor

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Options respresents options to start process
type Options struct {
	Command    string
	Args       []string
	Print      bool
	Wait       bool
	NewConsole bool
	Hide       bool
	OnChar     func(c string)
	OnLine     func(l string)
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

	cmd := exec.Command(opts.Command, opts.Args...)

	// Fix "ERROR: Input redirection is not supported, exiting the process immediately" on Windows:
	cmd.Stdin = os.Stdin

	if opts.NewConsole || opts.Hide {
		setCmdAttr(cmd, opts.NewConsole, opts.Hide)

		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
	} else { // Can capture output...
		stdoutReader, err := cmd.StdoutPipe()

		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return res
		}

		// Redirect StdErr to StdOut:
		cmd.Stderr = cmd.Stdout

		stdoutScanner := bufio.NewScanner(stdoutReader)
		stdoutScanner.Split(bufio.ScanRunes)
		var lineSb strings.Builder

		// Scan output:
		go func() {
			for stdoutScanner.Scan() {
				char := stdoutScanner.Text()
				if opts.Print {
					fmt.Print(char)
				}
				outSb.WriteString(char)
				// Char callback:
				if opts.OnChar != nil {
					opts.OnChar(char)
				}
				// Build the line:
				if opts.OnLine != nil {
					if char != "\n" {
						lineSb.WriteString(char)
					} else {
						// Line callback:
						opts.OnLine(lineSb.String())
						lineSb.Reset()
					}
				}
			}
		}()
	}

	err = cmd.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return res
	}

	if opts.Wait {
		err = cmd.Wait()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return res
		}
	}

	res.ExitCode = cmd.ProcessState.ExitCode()
	res.Output = outSb.String()

	return res
}
