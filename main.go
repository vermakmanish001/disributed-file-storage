package main

import (
	"bytes"
	"log"
	"time"

	"github.com/vermakmanish001/distributed-file-storage/p2p"
)

func makeServer(listenAddr string, nodes ...string) *FileServer {
	tcpTransport := p2p.NewTCPTransport(p2p.TCPTransportOpts{
		ListenAddr:    listenAddr,
		HandshakeFunc: p2p.NOPHandshakeFunc,
		Decoder:       &p2p.DefaultDecoder{},
		OnPeer:        nil,
	})

	fileServerOpts := FileServerOpts{
		ListenAddr:        listenAddr,
		StorageRoot:       listenAddr + "_network",
		PathTransformFunc: CASPathTransformFunc,
		Transport:         tcpTransport,
		BootstrapNodes:    nodes,
	}

	s := NewFileServer(fileServerOpts)
	tcpTransport.OnPeer = s.OnPeer
	return s
}

func main() {
	s1 := makeServer(":3000")
	s2 := makeServer(":4000", ":3000")

	go func() {
		log.Fatal(s1.Start())
	}()

	time.Sleep(4 * time.Second)

	go func() {
		log.Fatal(s2.Start())
	}()

	time.Sleep(4 * time.Second)

	data := bytes.NewReader([]byte("my big data file here "))
	s2.StoreData("myprivatedata", data)

	select {}
}
