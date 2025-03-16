//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/SCP002/executor"
	"golang.org/x/text/encoding/charmap"
)

func main() {
	ctx := context.Background()
	dirCmd := executor.NewCommand(ctx, executor.CmdOptions{
		Command: "cmd.exe",
		Args:    []string{"/C", "dir", "C:\\"},
	})

	findStrCmd := executor.NewCommand(ctx, executor.CmdOptions{
		Command: "cmd.exe",
		Args:    []string{"/C", "findstr", "Program"},
	})

	dirCmd.PipeStdoutTo(findStrCmd)

	_, err := dirCmd.Start(executor.StartOptions{
		Encoding: charmap.CodePage866,
	})
	if err != nil {
		panic(err)
	}

	res, err := findStrCmd.Start(executor.StartOptions{
		Wait:     true,
		Print:    true,
		Capture:  true,
		Encoding: charmap.CodePage866,
		OnChar:   func(c string, p *os.Process) {},
		OnLine:   func(l string, p *os.Process) {},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(strings.Repeat("-", 50))
	fmt.Println("\033[92mCaptured output will be displayed below:\033[0m")
	fmt.Print(res.Output)
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("\033[92mExit code: %v\033[0m\n", res.ExitCode)
}
