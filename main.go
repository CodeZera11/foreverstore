package main

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"github.com/codezera11/foreverstore/p2p"
)

func makeServer(listenAddr string, nodes ...string) *FileServer {
	tcpTransportOpts := p2p.TCPTransferOpts{
		ListenAddress: listenAddr,
		ShakeHands:    p2p.DefHandshakeFunc,
		Decoder:       p2p.DefaultDecoder{},
		// TODO: OnPeer func
		// OnPeer: ,
	}

	tcpTransport := p2p.NewTCPTransport(tcpTransportOpts)

	fileServerOpts := FileServerOpts{
		PathTransformFunc: CASPathTransformFunc,
		StorageRoot:       listenAddr + "_network",
		Transport:         tcpTransport,
		BootstrapNodes:    nodes,
	}

	s := NewServer(fileServerOpts)

	tcpTransport.OnPeer = s.OnPeer

	return s
}

func main() {

	s1 := makeServer(":3000", "")
	s2 := makeServer(":4000", ":3000")

	go func() {
		log.Fatal(s1.Start())
	}()
	time.Sleep(time.Second * 2)

	go s2.Start()
	time.Sleep(time.Second * 2)

	for i := 0; i < 10; i++ {
		data := bytes.NewReader([]byte("This is some beefy file!"))
		s2.Store(fmt.Sprintf("myprivatekey_%d", i), data)
		time.Sleep(time.Millisecond * 5)
	}

	// r, err := s2.Get("myprivatekey")

	// if err != nil {
	// 	log.Println(err)
	// }

	// b, err := io.ReadAll(r)

	// if err != nil {
	// 	log.Println(err)
	// }

	// fmt.Printf("File found with content: %v\n", string(b))

	select {}
}
