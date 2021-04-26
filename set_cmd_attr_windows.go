// +build windows

package executor

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// setCmdAttr sets OS specific process attributes
func setCmdAttr(cmd *exec.Cmd, newConsole bool, hide bool) {
	attr := syscall.SysProcAttr{}

	if newConsole {
		attr.CreationFlags |= windows.CREATE_NEW_CONSOLE
		attr.NoInheritHandles = true
	}

	if hide {
		attr.HideWindow = true
	}

	cmd.SysProcAttr = &attr
}
