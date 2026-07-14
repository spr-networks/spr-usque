//go:build !linux

package main

import (
	"fmt"
	"syscall"
)

func interfaceControl(interfaceName string) func(string, string, syscall.RawConn) error {
	return func(_, _ string, _ syscall.RawConn) error {
		return fmt.Errorf("binding diagnostics to %s requires Linux", interfaceName)
	}
}
