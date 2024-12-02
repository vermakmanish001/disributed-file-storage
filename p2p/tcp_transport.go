// package p2p

// import (
// 	"errors"
// 	"fmt"
// 	"log"
// 	"net"
// )

// // TCPPeer represents the remote node over a TCP established connection.
// type TCPPeer struct {
// 	// The underlying connection of the peer. Which in this case
// 	// is a TCP connection.
// 	conn net.Conn
// 	// if we dial and retrieve a conn => outbound == true
// 	// if we accept and retrieve a conn => outbound == false
// 	outbound bool
// }

// func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
// 	return &TCPPeer{
// 		conn:     conn,
// 		outbound: outbound,
// 	}
// }

// func (p *TCPPeer) RemoteAddr() net.Addr {
// 	return p.conn.RemoteAddr()
// }

// func (p *TCPPeer) Close() error {
// 	return p.conn.Close()
// }

// type TCPTransportOpts struct {
// 	ListenAddr    string
// 	HandshakeFunc HandshakeFunc
// 	Decoder       Decoder
// 	OnPeer        func(Peer) error
// }

// type TCPTransport struct {
// 	TCPTransportOpts
// 	listener net.Listener
// 	rpcch    chan RPC
// }

// func NewTCPTransport(opts TCPTransportOpts) *TCPTransport {
// 	return &TCPTransport{
// 		TCPTransportOpts: opts,
// 		rpcch:            make(chan RPC, 1024),
// 	}
// }

// func (t *TCPTransport) Consume() <-chan RPC {
// 	return t.rpcch
// }

// func (t *TCPTransport) Close() error {
// 	return t.listener.Close()
// }

// func (t *TCPTransport) Dial(addr string) error {
// 	conn, err := net.Dial("tcp", addr)
// 	if err != nil {
// 		return err
// 	}

// 	go t.handleConn(conn, true)

// 	return nil
// }

// func (t *TCPTransport) ListenAndAccept() error {
// 	var err error

// 	t.listener, err = net.Listen("tcp", t.ListenAddr)
// 	if err != nil {
// 		return err
// 	}

// 	go t.startAcceptLoop()

// 	log.Printf("listening at port %s\n", t.ListenAddr)

// 	return nil
// }

// func (t *TCPTransport) startAcceptLoop() {
// 	for {
// 		conn, err := t.listener.Accept()
// 		if errors.Is(err, net.ErrClosed) {
// 			return
// 		}

// 		if err != nil {
// 			fmt.Printf("TCP accept error: %s\n", err)
// 		}

// 		go t.handleConn(conn, false)
// 	}
// }

// type Temp struct{}

// func (t *TCPTransport) handleConn(conn net.Conn, outbound bool) {

// 	var err error

// 	defer func() {
// 		fmt.Printf("dropping peer connection: %s", err)
// 		conn.Close()
// 	}()

// 	peer := NewTCPPeer(conn, outbound)

// 	if err := t.HandshakeFunc(peer); err != nil {
// 		return
// 	}

// 	if t.OnPeer != nil {
// 		if err = t.OnPeer(peer); err != nil {
// 			return
// 		}
// 	}

// 	rpc := RPC{}

// 	for {
// 		// Use the Decoder to decode messages
// 		err = t.Decoder.Decode(conn, &rpc)
// 		if err != nil {
// 			return
// 		}
// 		rpc.From = conn.RemoteAddr()
// 		// Successfully decoded message
// 		t.rpcch <- rpc
// 	}
// }

package p2p

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
)

// TCPPeer represents the remote node over a TCP established connection.
type TCPPeer struct {
	net.Conn
	outbound bool

	Wg *sync.WaitGroup
}

func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		Conn:     conn,
		outbound: outbound,
		Wg:       &sync.WaitGroup{},
	}
}

func (p *TCPPeer) Send(b []byte) error {
	_, err := p.Conn.Write(b)
	return err
}

type TCPTransportOpts struct {
	ListenAddr    string
	HandshakeFunc HandshakeFunc
	Decoder       Decoder
	OnPeer        func(Peer) error
}

type TCPTransport struct {
	TCPTransportOpts
	listener net.Listener
	rpcch    chan RPC
}

func NewTCPTransport(opts TCPTransportOpts) *TCPTransport {
	return &TCPTransport{
		TCPTransportOpts: opts,
		rpcch:            make(chan RPC, 1024),
	}
}

func (t *TCPTransport) Consume() <-chan RPC {
	return t.rpcch
}

func (t *TCPTransport) Close() error {
	if t.listener != nil {
		return t.listener.Close()
	}
	return nil
}

func (t *TCPTransport) Dial(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}

	go t.handleConn(conn, true)
	return nil
}

func (t *TCPTransport) ListenAndAccept() error {
	var err error
	t.listener, err = net.Listen("tcp", t.ListenAddr)
	if err != nil {
		return err
	}

	go t.startAcceptLoop()
	log.Printf("Listening at port %s\n", t.ListenAddr)
	return nil
}

func (t *TCPTransport) startAcceptLoop() {
	for {
		conn, err := t.listener.Accept()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		if err != nil {
			log.Printf("TCP accept error: %s", err)
			continue
		}

		go t.handleConn(conn, false)
	}
}

func (t *TCPTransport) handleConn(conn net.Conn, outbound bool) {
	var err error
	defer func() {
		if err != nil {
			log.Printf("Dropping peer connection: %s", err)
		}
		conn.Close()
	}()

	peer := NewTCPPeer(conn, outbound)

	if err = t.HandshakeFunc(peer); err != nil {
		log.Printf("Handshake failed for peer %s: %v", conn.RemoteAddr(), err)
		return
	}

	if t.OnPeer != nil {
		if err = t.OnPeer(peer); err != nil {
			log.Printf("OnPeer callback failed for peer %s: %v", conn.RemoteAddr(), err)
			return
		}
	}

	rpc := RPC{}
	for {
		err = t.Decoder.Decode(conn, &rpc)
		if err != nil {
			log.Printf("Decode error from %s: %v", conn.RemoteAddr(), err)
			return
		}

		rpc.From = conn.RemoteAddr().String()
		peer.Wg.Add(1)
		fmt.Println("waiting till stream is done")
		t.rpcch <- rpc
		peer.Wg.Wait()
		fmt.Println("stream done continueing normal read loop")

	}
}
