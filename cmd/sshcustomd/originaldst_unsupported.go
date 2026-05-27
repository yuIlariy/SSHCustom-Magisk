//go:build !linux

package main

import (
	"context"
	"errors"
	"net"
)

func originalDst(conn *net.TCPConn) (string, error) {
	return "", errors.New("transparent original-dst is only supported on linux")
}

func tproxyOriginalDst(conn net.Conn) (string, error) {
	return "", errors.New("tproxy is only supported on linux")
}

func listenTPROXY(ctx context.Context, address string) (net.Listener, error) {
	return nil, errors.New("tproxy is only supported on linux")
}
