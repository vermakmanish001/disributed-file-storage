package main

import (
	"fmt"
	"log"

	"github.com/vermakmanish001/distributed-file-storage/p2p"
)

func OnPeer(peer p2p.Peer) error {
	peer.Close()
	return nil
}

func main() {

	tcpOpts := p2p.TCPTransportOpts{
		ListenAddr:    ":3000",
		HandshakeFunc: p2p.NOPHandshakeFunc,
		Decoder:       p2p.DefaultDecoder{},
		OnPeer:        OnPeer,
	}

	// Pass the tcpOpts variable to the NewTCPTransport function
	tr := p2p.NewTCPTransport(tcpOpts)

	go func() {
		for {
			msg := <-tr.Consume()
			fmt.Printf("%v\n", msg)
		}
	}()

	// Start listening and accepting connections
	if err := tr.ListenAndAccept(); err != nil {
		log.Fatal(err)
	}

	// Keep the main function running
	select {}
}
