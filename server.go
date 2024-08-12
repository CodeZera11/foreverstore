package main

import (
	"log"

	"github.com/codezera11/foreverstore/p2p"
)

type FileServerOpts struct {
	PathTransformFunc PathTransformFunc
	Transport         p2p.Transport
	StorageRoot       string
}

type FileServer struct {
	FileServerOpts
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
	}
}

func (f *FileServer) Start() error {
	err := f.Transport.ListenAndAccept()

	if err != nil {
		return err
	}

	f.loop()

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

func (f *FileServer) Stop() {
	close(f.quitch)
}
