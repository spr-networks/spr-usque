//go:build linux

package main

import "syscall"

func interfaceControl(interfaceName string) func(string, string, syscall.RawConn) error {
	return func(_, _ string, raw syscall.RawConn) error {
		var sockErr error
		if err := raw.Control(func(fd uintptr) {
			sockErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, interfaceName)
		}); err != nil {
			return err
		}
		return sockErr
	}
}
