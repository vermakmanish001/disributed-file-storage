package p2p

import "net"

// Peer represents a remote node in the network.
type Peer interface {
	net.Conn
	Send([]byte) error
	CloseStream()
}

// Transport handles communication between nodes (e.g., TCP, UDP).
type Transport interface {
	Addr() string
	Dial(string) error
	ListenAndAccept() error
	Consume() <-chan RPC
	Close() error
}
