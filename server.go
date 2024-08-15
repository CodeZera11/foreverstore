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

	gob.Register(MessageStoreFile{})

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

type MessageStoreFile struct {
	Key  string
	Size int64
}

type DataMessage struct {
	Key  string
	Data []byte
}

func (f *FileServer) StoreData(key string, r io.Reader) error {

	var (
		fileBuffer = new(bytes.Buffer)
		tee        = io.TeeReader(r, fileBuffer)
	)

	size, err := f.store.Write(key, tee)

	if err != nil {
		return err
	}

	msgBuf := new(bytes.Buffer)
	msg := Message{
		Payload: MessageStoreFile{
			Key:  key,
			Size: size,
		},
	}

	if err := gob.NewEncoder(msgBuf).Encode(msg); err != nil {
		return err
	}

	for _, peer := range f.peers {
		if err := peer.Send(msgBuf.Bytes()); err != nil {
			return err
		}
	}

	time.Sleep(time.Second * 1)

	for _, peer := range f.peers {
		n, err := io.Copy(peer, fileBuffer)
		if err != nil {
			return err
		}

		fmt.Printf("recd and sent %v bytes to disk\n", n)
	}

	return nil
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
		case rpc := <-f.Transport.Consume():
			var m Message
			if err := gob.NewDecoder(bytes.NewReader(rpc.Payload)).Decode(&m); err != nil {
				log.Println(err)
			}
			if err := f.handleMessage(rpc.From, &m); err != nil {
				log.Println(err)
				return
			}

		case <-f.quitch:
			return
		}
	}
}

func (f *FileServer) handleMessage(from string, msg *Message) error {
	switch v := msg.Payload.(type) {
	case MessageStoreFile:
		return f.handleMessageStoreFile(from, v)
	}

	return nil
}

func (f *FileServer) handleMessageStoreFile(from string, msg MessageStoreFile) error {

	fmt.Printf("from %s and msg %v\n", from, msg)

	peer, ok := f.peers[from]
	if !ok {
		return fmt.Errorf("peer %s not found in the peer map", from)
	}

	n, err := f.store.Write(msg.Key, peer)

	if err != nil {
		return nil
	}

	fmt.Printf("Written %v bytes to disk\n", n)

	peer.(*p2p.TCPPeer).Wg.Done()

	return nil

}
