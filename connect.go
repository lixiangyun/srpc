package srpc

import (
	"net"
)

const (
	TYPE_TCP  = "tcp"
	TYPE_UDP  = "udp"
	TYPE_UNIX = "unix"
)

type connect struct {
	network string
	addr    string
	conn    net.Conn
}
