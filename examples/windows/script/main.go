//go:build windows

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/SCP002/executor"
)

func main() {
	opts := executor.Options{
		Command: ".\\script.cmd",
		Args:    []string{"arg1"},
		Wait:    true,
		Print:   true,
		Capture: true,
		OnChar: func(c string, p *os.Process) {},
		OnLine: func(l string, p *os.Process) {},
	}

	res, err := executor.Start(opts)
	if err != nil {
		panic(err)
	}

	fmt.Println(strings.Repeat("-", 50))
	fmt.Println("\033[92mCaptured output will be displayed below:\033[0m")
	fmt.Print(res.Output)
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("\033[92mExit code: %v\033[0m\n", res.ExitCode)
}
