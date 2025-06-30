//go:build darwin

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

	var pid int32 = -1
	var uid uint32 = 0

	err = rawConn.Control(func(fd uintptr) {
		// GetsockoptInt is used to get PID of the process connected on the other end of the UDS.
		// See https://github.com/golang/sys/blob/master/unix/syscall_unix.go#L276
		pid_, err := unix.GetsockoptInt(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERPID)
		if err != nil {
			return
		}
		pid = int32(pid_)

		// GetsockoptXucred is used to get UID of the process connected on the other end of the UDS
		// See https://github.com/golang/sys/blob/master/unix/syscall_darwin.go#L486
		xucredVal, err := unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if err != nil {
			return
		}
		uid = xucredVal.Uid
		return
	})

	return UCred{pid, uid}, err
}
