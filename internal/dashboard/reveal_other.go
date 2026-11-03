//go:build !windows

package dashboard

import "syscall"

func explorerSelectAttr(path string) *syscall.SysProcAttr { return nil }
