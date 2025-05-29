//go:build linux

package proxy

import (
	"net"
	"golang.org/x/sys/unix"
)

func getUdsPeerCredentialsOS(conn *net.UnixConn) (UCred, error) {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return UCred{-1, 0}, err
	}

	var peerCred *unix.Ucred
	var credErr error
	err = rawConn.Control(func(fd uintptr) {
		// GetsockoptUcred is used to retrieve the credentials (PID, UID, GID)
		// of the process connected on the other end of the UDS.
		// See https://github.com/golang/sys/blob/master/unix/syscall_linux.go#L1285
		peerCred, credErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	})

	if credErr != nil {
		return UCred{-1, 0}, credErr
	}
	return UCred{peerCred.Pid, peerCred.Uid}, nil
}
