package main

import (
	"log"
	"time"

	"github.com/codezera11/foreverstore/p2p"
)

func main() {

	tcpTransportOpts := p2p.TCPTransferOpts{
		ListenAddress: ":3000",
		ShakeHands:    p2p.DefHandshakeFunc,
		Decoder:       p2p.DefaultDecoder{},
	}

	fileServerOpts := FileServerOpts{
		PathTransformFunc: CASPathTransformFunc,
		StorageRoot:       "3000_network",
		Transport:         p2p.NewTCPTransport(tcpTransportOpts),
	}

	s := NewServer(fileServerOpts)

	go func() {
		time.Sleep(time.Second * 4)
		s.Stop()
	}()

	if err := s.Start(); err != nil {
		log.Fatal(err)
	}

}
