// package main

// import (
// 	"fmt"
// 	"log"
// 	"sync"

// 	"github.com/vermakmanish001/distributed-file-storage/p2p"
// )

// type FileServerOpts struct {
// 	ListenAddr        string
// 	StorageRoot       string
// 	PathTransformFunc PathTransformFunc
// 	Transport         p2p.Transport
// 	BootstrapNodes    []string
// }

// type FileServer struct {
// 	FileServerOpts

// 	peerLock sync.Mutex
// 	peers    map[string]p2p.Peer
// 	store    *Store
// 	quitch   chan struct{}
// }

// func NewFileServer(opts FileServerOpts) *FileServer {
// 	storeOpts := StoreOpts{
// 		Root:              opts.StorageRoot,
// 		PathTransformFunc: opts.PathTransformFunc,
// 	}

// 	return &FileServer{
// 		FileServerOpts: opts,
// 		store:          NewStore(storeOpts),
// 		quitch:         make(chan struct{}),
// 		peers:          make(map[string]p2p.Peer),
// 	}
// }

// func (s *FileServer) Stop() {
// 	close(s.quitch)
// }

// func (s *FileServer) OnPeer(p p2p.Peer) error {
// 	s.peerLock.Lock()
// 	defer s.peerLock.Unlock()

// 	s.peers[p.RemoteAddr().String()] = p

// 	log.Printf("connected with remote %s", p.RemoteAddr())

// 	return nil
// }

// func (s *FileServer) loop() {

// 	defer func() {
// 		log.Println("file server stopped due to error or user quit action")
// 		s.Transport.Close()
// 	}()

// 	for {
// 		select {
// 		case msg := <-s.Transport.Consume():
// 			fmt.Println(msg)
// 		case <-s.quitch:
// 			return
// 		}
// 	}
// }

// func (s *FileServer) bootstrapNetwork() error {
// 	for _, addr := range s.BootstrapNodes {
// 		if len(addr) == 0 {
// 			continue
// 		}

// 		go func(addr string) {
// 			fmt.Println("attempting to connect with remote", addr)
// 			if err := s.Transport.Dial(addr); err != nil {
// 				log.Println("dial error: ", err)
// 			}
// 		}(addr)
// 	}

// 	return nil
// }

// func (s *FileServer) Start() error {

// 	if err := s.Transport.ListenAndAccept(); err != nil {
// 		return err
// 	}
// 	s.bootstrapNetwork()
// 	s.loop()

// 	return nil
// }

package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/vermakmanish001/distributed-file-storage/p2p"
)

type FileServerOpts struct {
	ListenAddr        string
	StorageRoot       string
	PathTransformFunc PathTransformFunc
	Transport         p2p.Transport
	BootstrapNodes    []string
}

type FileServer struct {
	FileServerOpts

	peerLock sync.Mutex
	peers    map[string]p2p.Peer
	store    *Store
	quitch   chan struct{}
}

func NewFileServer(opts FileServerOpts) *FileServer {
	storeOpts := StoreOpts{
		Root:              opts.StorageRoot,
		PathTransformFunc: opts.PathTransformFunc,
	}

	return &FileServer{
		FileServerOpts: opts,
		store:          NewStore(storeOpts),
		quitch:         make(chan struct{}),
		peers:          make(map[string]p2p.Peer),
	}
}

func (s *FileServer) broadcast(msg *Message) error {

	peers := []io.Writer{}
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}

	mw := io.MultiWriter(peers...)
	return gob.NewEncoder(mw).Encode(msg)
}

type Message struct {
	Payload any
}

type MessageStoreFile struct {
	Key string
}

func (s *FileServer) StoreData(key string, r io.Reader) error {

	buf := new(bytes.Buffer)
	msg := Message{
		Payload: MessageStoreFile{
			Key: key,
		},
	}
	if err := gob.NewEncoder(buf).Encode(msg); err != nil {
		return err
	}
	for _, peer := range s.peers {
		if err := peer.Send(buf.Bytes()); err != nil {
			return err
		}
	}

	// time.Sleep(time.Second * 3)
	// payload := []byte("this is a largefile")
	// for _, peer := range s.peers {
	// 	if err := peer.Send(payload); err != nil {
	// 		return err
	// 	}
	// }

	return nil
	// buf := new(bytes.Buffer)
	// tee := io.TeeReader(r, buf)
	// if err := s.store.Write(key, tee); err != nil {

	// 	return err
	// }

	// p := &DataMessage{
	// 	Key:  key,
	// 	Data: buf.Bytes(),
	// }

	// fmt.Println(buf.Bytes())
	// return s.broadcast(&Message{
	// 	From:    "todo",
	// 	Payload: p,
	// })
}

func (s *FileServer) Stop() {
	close(s.quitch)
}

func (s *FileServer) OnPeer(p p2p.Peer) error {
	s.peerLock.Lock()
	defer s.peerLock.Unlock()

	s.peers[p.RemoteAddr().String()] = p
	log.Printf("Connected with remote peer: %s", p.RemoteAddr())
	return nil
}

func (s *FileServer) loop() {
	defer func() {
		log.Println("File server stopped due to error or user quit action")
		s.Transport.Close()
	}()

	for {
		select {
		case rpc := <-s.Transport.Consume():

			var msg Message
			if err := gob.NewDecoder(bytes.NewReader(rpc.Payload)).Decode(&msg); err != nil {
				log.Println(err)
				return
			}

			fmt.Printf("Payload: %+v\n", msg.Payload)
			peer, ok := s.peers[rpc.From]
			if !ok {
				panic("peer not found")
			}
			b := make([]byte, 1000)
			if _, err := peer.Read(b); err != nil {
				panic(err)
			}
			fmt.Printf("%s\n", string(b))

			peer.(*p2p.TCPPeer).Wg.Done()

			// if err := s.handleMessage(&m); err != nil {
			// 	log.Println(err)
			// }

		case <-s.quitch:
			return
		}
	}
}

// func (s *FileServer) handleMessage(msg *Message) error {

// 	switch v := msg.Payload.(type) {
// 	case *DataMessage:
// 		fmt.Printf("received data %+v\n", v)
// 	}
// 	return nil
// }

func (s *FileServer) bootstrapNetwork() error {
	for _, addr := range s.BootstrapNodes {
		if len(addr) == 0 {
			continue
		}

		go func(addr string) {
			log.Printf("Attempting to connect with remote: %s", addr)
			if err := s.Transport.Dial(addr); err != nil {
				log.Printf("Dial error with %s: %v", addr, err)
			} else {
				log.Printf("Successfully connected to %s", addr)
			}
		}(addr)
	}

	return nil
}

func (s *FileServer) Start() error {
	log.Printf("Starting server on %s", s.ListenAddr)

	if err := s.Transport.ListenAndAccept(); err != nil {
		return err
	}

	log.Println("Server listening and accepting connections")
	s.bootstrapNetwork()
	s.loop()

	return nil
}

func init() {
	gob.Register(MessageStoreFile{})
}
