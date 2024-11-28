package main

import (
	"log"

	"github.com/vermakmanish001/distributed-file-storage/p2p"
)

func main() {

	tcpOpts := p2p.TCPTransportOpts{
		ListenAddr:    ":3000",
		HandshakeFunc: p2p.NOPHandshakeFunc,
		Decoder:       p2p.DefaultDecoder{},
	}

	// Pass the tcpOpts variable to the NewTCPTransport function
	tr := p2p.NewTCPTransport(tcpOpts)

	// Start listening and accepting connections
	if err := tr.ListenAndAccept(); err != nil {
		log.Fatal(err)
	}

	// Keep the main function running
	select {}
}
