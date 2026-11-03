//go:build windows

package dashboard

import "syscall"

// explorerSelectAttr makes explorer.exe open the folder with path
// selected. The raw command line is set by hand because explorer wants
// `/select,"C:\path with spaces\file.pdf"` as one token.
func explorerSelectAttr(path string) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + path + `"`}
}
