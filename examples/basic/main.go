package main

import (
	"fmt"
	"strings"

	"github.com/SCP002/executor"
)

func main() {
	opts := executor.Options{
		Command: "..\\..\\assets\\sample-executable.cmd",
		Args:    []string{"arg1"},
		Wait:    true,
		Print:   true,
		Capture: true,
	}

	res := executor.Start(opts)

	fmt.Println(strings.Repeat("-", 30))
	fmt.Print(res.Output)

	fmt.Println("Press <Enter> to exit...")
	_, _ = fmt.Scanln()
}
