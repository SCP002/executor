// +build windows

package executor

import (
	"os/exec"
	"syscall"
)

// setCmdAttr sets OS specific process attributes
func setCmdAttr(cmd *exec.Cmd, newConsole bool, hide bool) {
	attr := syscall.SysProcAttr{}

	if newConsole {
		attr.CreationFlags |= 0x00000010 // CREATE_NEW_CONSOLE
		attr.NoInheritHandles = true
	}

	if hide {
		attr.HideWindow = true
	}

	cmd.SysProcAttr = &attr
}
