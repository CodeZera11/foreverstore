package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/codezera11/foreverstore/p2p"
)

type FileServerOpts struct {
	PathTransformFunc PathTransformFunc
	Transport         p2p.Transport
	StorageRoot       string
	BootstrapNodes    []string
}

type FileServer struct {
	FileServerOpts

	peerLock sync.Mutex
	peers    map[string]p2p.Peer
	store    *Store
	quitch   chan struct{}
}

func NewServer(opts FileServerOpts) *FileServer {

	storeOpts := StoreOpts{
		Root:              opts.StorageRoot,
		PathTransformFunc: opts.PathTransformFunc,
	}

	return &FileServer{
		store:          NewStore(storeOpts),
		FileServerOpts: opts,
		quitch:         make(chan struct{}),
		peers:          make(map[string]p2p.Peer),
	}
}

func (f *FileServer) Start() error {
	err := f.Transport.ListenAndAccept()

	if err != nil {
		return err
	}

	f.bootstrapNetwork()
	f.loop()

	return nil
}

func (f *FileServer) Stop() {
	close(f.quitch)
}

func (f *FileServer) OnPeer(p p2p.Peer) error {
	f.peerLock.Lock()
	defer f.peerLock.Unlock()

	f.peers[p.RemoteAddr().String()] = p

	log.Printf("connected with remote %s\n", p.RemoteAddr())

	return nil
}

func (f *FileServer) bootstrapNetwork() error {
	for _, addr := range f.BootstrapNodes {
		if len(addr) == 0 {
			continue
		}
		go func(addr string) {
			fmt.Println("attempting to connect with remote: ", addr)
			if err := f.Transport.Dial(addr); err != nil {
				log.Println("Dial error:", err)
			}
		}(addr)
	}

	return nil
}

func (f *FileServer) loop() {
	defer func() {
		log.Printf("file server stopped due to user quit action")
		f.Transport.Close()
	}()
	for {
		select {
		case msg := <-f.Transport.Consume():
			log.Println(string(msg.Payload))
		case <-f.quitch:
			return
		}
	}
}
