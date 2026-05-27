//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"syscall"
	"unsafe"
)

const soOriginalDst = 80

// originalDst reads the original destination from a REDIRECT'd connection
// using SO_ORIGINAL_DST. This is the legacy path kept for compatibility.
func originalDst(conn *net.TCPConn) (string, error) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return "", err
	}
	var out string
	var serr error
	err = raw.Control(func(fd uintptr) {
		var addr syscall.RawSockaddrInet4
		sz := uint32(unsafe.Sizeof(addr))
		_, _, errno := syscall.Syscall6(syscall.SYS_GETSOCKOPT, fd, uintptr(syscall.SOL_IP), uintptr(soOriginalDst), uintptr(unsafe.Pointer(&addr)), uintptr(unsafe.Pointer(&sz)), 0)
		if errno != 0 {
			serr = errno
			return
		}
		if addr.Family != syscall.AF_INET {
			serr = fmt.Errorf("unexpected original dst family %d", addr.Family)
			return
		}
		port := int((addr.Port&0xff)<<8 | addr.Port>>8)
		ip := net.IPv4(addr.Addr[0], addr.Addr[1], addr.Addr[2], addr.Addr[3]).String()
		out = net.JoinHostPort(ip, strconv.Itoa(port))
	})
	if err != nil {
		return "", err
	}
	if serr != nil {
		return "", serr
	}
	if out == "" {
		return "", errors.New("empty original dst")
	}
	return out, nil
}

// tproxyOriginalDst extracts the original destination from a TPROXY'd connection.
// With TPROXY, the kernel preserves the original destination as the local address
// of the accepted connection — no getsockopt needed.
func tproxyOriginalDst(conn net.Conn) (string, error) {
	addr := conn.LocalAddr()
	if addr == nil {
		return "", errors.New("nil local address")
	}
	return addr.String(), nil
}

// listenTPROXY creates a TCP listener with IP_TRANSPARENT socket option.
// This allows accepting connections destined for arbitrary IP addresses
// (as redirected by TPROXY iptables rules). The listener's accepted
// connections will have LocalAddr() set to the original destination.
func listenTPROXY(ctx context.Context, address string) (net.Listener, error) {
	lc := net.ListenConfig{
		Control: func(network, addr string, c syscall.RawConn) error {
			var opErr error
			err := c.Control(func(fd uintptr) {
				// IP_TRANSPARENT (19) allows binding to non-local addresses
				// and accepting TPROXY'd connections.
				opErr = syscall.SetsockoptInt(int(fd), syscall.SOL_IP, syscall.IP_TRANSPARENT, 1)
				if opErr != nil {
					return
				}
				// SO_REUSEADDR for quick restart
				opErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			})
			if err != nil {
				return err
			}
			return opErr
		},
	}
	return lc.Listen(ctx, "tcp", address)
}
