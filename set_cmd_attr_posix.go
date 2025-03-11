//go:build !windows

package executor

import (
	"os/exec"
)

// setCmdAttr sets OS specific process attributes
func setCmdAttr(cmd *exec.Cmd, newConsole bool, hide bool) {}
