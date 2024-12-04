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
	"time"

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

func (s *FileServer) stream(msg *Message) error {

	peers := []io.Writer{}
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}

	mw := io.MultiWriter(peers...)
	return gob.NewEncoder(mw).Encode(msg)
}

func (s *FileServer) broadcast(msg *Message) error {

	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(msg); err != nil {
		return err
	}

	for _, peer := range s.peers {
		peer.Send([]byte{p2p.IncomingMessage})
		if err := peer.Send(buf.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

type Message struct {
	Payload any
}

type MessageStoreFile struct {
	Key  string
	Size int64
}

type MessageGetFile struct {
	Key string
}

func (s *FileServer) Get(key string) (io.Reader, error) {

	if s.store.Has(key) {
		return s.store.Read(key)
	}
	fmt.Printf("dont have file (%s) locally, fetching from network...\n", key)

	msg := Message{
		Payload: MessageGetFile{
			Key: key,
		},
	}

	if err := s.broadcast(&msg); err != nil {
		return nil, err
	}

	for _, peer := range s.peers {
		fileBuffer := new(bytes.Buffer)
		n, err := io.Copy(fileBuffer, peer)
		if err != nil {
			return nil, err
		}
		fmt.Println("received bytes over the network :", n)
		fmt.Println(fileBuffer.String())
	}

	select {}
	return nil, nil
}

func (s *FileServer) Store(key string, r io.Reader) error {

	var (
		fileBuffer = new(bytes.Buffer)
		tee        = io.TeeReader(r, fileBuffer)
	)
	size, err := s.store.Write(key, tee)
	if err != nil {
		return err
	}
	msg := Message{
		Payload: MessageStoreFile{
			Key:  key,
			Size: size,
		},
	}

	if err := s.broadcast(&msg); err != nil {
		return err
	}

	time.Sleep(time.Millisecond * 5)

	for _, peer := range s.peers {
		peer.Send([]byte{p2p.IncomingStream})
		n, err := io.Copy(peer, fileBuffer)
		if err != nil {
			return err
		}
		fmt.Println("received and written bytes to disk", n)
	}

	return nil

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
				log.Println("decoding error: ", err)

			}

			if err := s.handleMessage(rpc.From, &msg); err != nil {
				log.Println("handling message error: ", err)

			}

		case <-s.quitch:
			return
		}
	}
}

func (s *FileServer) handleMessage(from string, msg *Message) error {

	switch v := msg.Payload.(type) {
	case MessageStoreFile:
		return s.handleMessageStoreFile(from, v)
	case MessageGetFile:
		return s.handleMessageGetFile(from, v)
	}
	return nil

}

func (s *FileServer) handleMessageGetFile(from string, msg MessageGetFile) error {
	if !s.store.Has(msg.Key) {
		return fmt.Errorf("need to server file (%s) does not exist on disk", msg.Key)
	}

	fmt.Printf("serving file (%s) over the network\n", msg.Key)
	r, err := s.store.Read(msg.Key)
	if err != nil {
		return err
	}

	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("peer %s not in map", from)
	}

	n, err := io.Copy(peer, r)
	if err != nil {
		return err
	}

	fmt.Printf("written (%d) bytes over the network to %s\n", n, from)

	return nil
}

func (s *FileServer) handleMessageStoreFile(from string, msg MessageStoreFile) error {
	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("peer (%s) could not be found in peer list", from)
	}

	n, err := s.store.Write(msg.Key, io.LimitReader(peer, msg.Size))
	if err != nil {
		return err
	}

	fmt.Printf("written %d bytes to disk\n", n)

	peer.(*p2p.TCPPeer).Wg.Done()

	return nil
}

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
	gob.Register(MessageGetFile{})
}
