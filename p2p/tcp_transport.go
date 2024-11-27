package p2p

import (
	"net"
	"sync"
)

type TCPTransport struct {
	ListenAddr string
	listener   net.Listener

	mu   sync.RWMutex
	peer map[net.Addr]Peer
}

func NewTCPTransport(listenAddr string) *TCPTransport {
	return &TCPTransport{
		listenAddress: listenAddr,
	}
}
