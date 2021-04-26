package main

import (
	"fmt"
	"strings"

	"github.com/SCP002/executor"
)

func main() {
	opts := executor.Options{
		Command:    "sample-executable.cmd",
		Args:       []string{"arg1"},
		Wait:       false,
		NewConsole: true,
	}

	res := executor.Start(opts)

	fmt.Println(strings.Repeat("-", 30))
	fmt.Print(res.Output)

	fmt.Println("Press <Enter> to exit...")
	fmt.Scanln()
}
