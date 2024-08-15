package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

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

	store  *Store
	quitch chan struct{}
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

	addr := p.RemoteAddr().String()
	f.peers[addr] = p
	log.Printf("Peer added: %s. Total peers: %d", addr, len(f.peers))

	return nil
}

type Message struct {
	From    string
	Payload any
}

type DataMessage struct {
	Key  string
	Data []byte
}

func (f *FileServer) StoreData(key string, r io.Reader) error {

	buf := new(bytes.Buffer)
	msg := Message{
		Payload: []byte("storagekey"),
	}

	if err := gob.NewEncoder(buf).Encode(msg); err != nil {
		return err
	}

	for _, peer := range f.peers {
		if err := peer.Send(buf.Bytes()); err != nil {
			return err
		}
	}

	time.Sleep(time.Second * 3)

	payload := []byte("THIS IS A LARGE FILE")
	for _, peer := range f.peers {
		if err := peer.Send(payload); err != nil {
			return err
		}
	}

	return nil

	// buf := new(bytes.Buffer)
	// tee := io.TeeReader(r, buf)

	// if err := f.store.Write(key, tee); err != nil {
	// 	return err
	// }

	// p := &DataMessage{
	// 	Key:  key,
	// 	Data: buf.Bytes(),
	// }

	// return f.broadcast(&Message{
	// 	From:    "todo",
	// 	Payload: p,
	// })
}

func (f *FileServer) broadcast(msg *Message) error {
	log.Printf("Broadcasting to %d peers", len(f.peers))

	peers := []io.Writer{}
	for _, peer := range f.peers {
		peers = append(peers, peer)
	}

	if len(peers) == 0 {
		return fmt.Errorf("no peers to broadcast to")
	}

	mw := io.MultiWriter(peers...)
	return gob.NewEncoder(mw).Encode(msg)
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
			var m Message
			if err := gob.NewDecoder(bytes.NewReader(msg.Payload)).Decode(&m); err != nil {
				log.Println(err)
			}
			fmt.Println("recd key:", string(m.Payload.([]byte)))

			peer, ok := f.peers[msg.From]
			if !ok {
				panic("peer not found in the peer map!")
			}

			b := make([]byte, 1000)
			if _, err := peer.Read(b); err != nil {
				panic(err)
			}

			fmt.Println("recd file:", string(b))

			peer.(*p2p.TCPPeer).Wg.Done()

		case <-f.quitch:
			return
		}
	}
}

func (f *FileServer) handleMessage(msg *Message) error {
	switch v := msg.Payload.(type) {
	case *DataMessage:
		fmt.Printf("Received data %s\n", v)
	}

	return nil
}
