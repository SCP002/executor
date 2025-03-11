package main

import (
	"fmt"
	// "os"
	"strings"

	"github.com/SCP002/executor"
)

func main() {
	opts := executor.Options{
		Command: ".\\sample-executable.cmd",
		Args:    []string{"arg1"},
		Wait:    true,
		Print:   true,
		Capture: true,
		// OnChar: func(c string, p *os.Process) {
		// 	fmt.Print(c)
		// },
	}

	res, err := executor.Start(opts)
	if err != nil {
		panic(err)
	}

	fmt.Println(strings.Repeat("-", 50))
	fmt.Print(res.Output)
}
