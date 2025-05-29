//go:build windows

package proxy

import (
	"fmt"
	"net"
)

func getUdsPeerCredentialsOS(conn *net.UnixConn) (UCred, error) {
	return UCred{-1, 0}, fmt.Errorf("Windows is currently not supported")
}
