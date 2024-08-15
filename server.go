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
	// From    string
	Payload any
}

type MessageStoreFile struct {
	Key  string
	Size int64
}

type MessageGetFile struct {
	Key string
}

func (f *FileServer) Get(key string) (io.Reader, error) {
	if f.store.Has(key) {
		return f.store.Read(key)
	}

	fmt.Printf("Don't have file locally! Trying to fetch file from the network...\n")

	msg := Message{
		Payload: MessageGetFile{
			Key: key,
		},
	}

	err := f.broadcast(&msg)
	if err != nil {
		return nil, err
	}

	time.Sleep(time.Millisecond * 100)

	for _, peer := range f.peers {
		fmt.Println("Receiving stream from peer:", peer.RemoteAddr())
		fileBuffer := new(bytes.Buffer)
		n, err := io.Copy(fileBuffer, peer)
		if err != nil {
			return nil, err
		}
		fmt.Println("received bytes over the network:", n)
		fmt.Println(fileBuffer.String())
	}

	select {}

	return nil, nil
}

func (f *FileServer) Store(key string, r io.Reader) error {
	var (
		fileBuffer = new(bytes.Buffer)
		tee        = io.TeeReader(r, fileBuffer)
	)

	size, err := f.store.Write(key, tee)
	if err != nil {
		return err
	}

	msg := Message{
		Payload: MessageStoreFile{
			Key:  key,
			Size: size,
		},
	}

	err = f.broadcast(&msg)
	if err != nil {
		return err
	}

	time.Sleep(time.Millisecond * 50)

	// TODO: use a multiwriter here
	for _, peer := range f.peers {
		peer.Send([]byte{p2p.IncomingStream})
		n, err := io.Copy(peer, fileBuffer)
		if err != nil {
			return err
		}

		fmt.Println("received and written bytes to disk:", n)
	}

	return nil
}

func (f *FileServer) broadcast(msg *Message) error {
	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(msg); err != nil {
		return err
	}

	for _, peer := range f.peers {
		peer.Send([]byte{p2p.IncomingMessage})
		err := peer.Send(buf.Bytes())
		if err != nil {
			return err
		}
	}

	return nil
}

// func (f *FileServer) stream(msg *Message) error {
// 	log.Printf("Broadcasting to %d peers", len(f.peers))

// 	peers := []io.Writer{}
// 	for _, peer := range f.peers {
// 		peers = append(peers, peer)
// 	}

// 	if len(peers) == 0 {
// 		return fmt.Errorf("no peers to broadcast to")
// 	}

// 	mw := io.MultiWriter(peers...)
// 	return gob.NewEncoder(mw).Encode(msg)
// }

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
		log.Printf("file server stopped due to error or user quit action")
		f.Transport.Close()
	}()
	for {
		select {
		case rpc := <-f.Transport.Consume():
			var msg Message
			if err := gob.NewDecoder(bytes.NewReader(rpc.Payload)).Decode(&msg); err != nil {
				log.Println("decoding error:", err)
			}
			if err := f.handleMessage(rpc.From, &msg); err != nil {
				log.Println("handle message error:", err)
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
	case MessageGetFile:
		return f.handleMessageGetFile(from, v)
	}

	return nil
}

func (f *FileServer) handleMessageStoreFile(from string, msg MessageStoreFile) error {
	peer, ok := f.peers[from]
	if !ok {
		return fmt.Errorf("peer %s not found in the peer map", from)
	}

	time.Sleep(time.Millisecond * 100)

	n, err := f.store.Write(msg.Key, io.LimitReader(peer, msg.Size))
	if err != nil {
		return nil
	}

	fmt.Printf("[%s] written %v bytes to disk\n", f.Transport.Addr(), n)
	peer.(*p2p.TCPPeer).Wg.Done()

	return nil
}

func (f *FileServer) handleMessageGetFile(from string, msg MessageGetFile) error {
	if !f.store.Has(msg.Key) {
		return fmt.Errorf("file with key %v not found on the network", msg.Key)
	}

	r, err := f.store.Read(msg.Key)
	if err != nil {
		return err
	}

	peer, ok := f.peers[from]
	if !ok {
		return fmt.Errorf("peer with addr %v not found on map", from)
	}

	n, err := io.Copy(peer, r)
	if err != nil {
		return err
	}

	fmt.Printf("written %d bytes over the network %s\n", n, from)
	return nil
}

func init() {
	gob.Register(MessageStoreFile{})
	gob.Register(MessageGetFile{})
}
