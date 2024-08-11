package main

import (
	"fmt"
	"log"

	"github.com/codezera11/foreverstore/p2p"
)

func OnPeer(peer p2p.Peer) error {
	peer.Close()
	// return fmt.Errorf("failed the onpeer func")
	fmt.Println("doing some logic with the peer outside of TCPTransport")
	return nil
}

func main() {

	opts := p2p.TCPTransferOpts{
		ListenAddress: ":3000",
		ShakeHands:    p2p.DefHandshakeFunc,
		Decoder:       p2p.DefaultDecoder{},
		OnPeer:        OnPeer,
	}

	tr := p2p.NewTCPTransport(opts)

	go func() {
		for {
			msgCh := tr.Consume()
			msg := <-msgCh
			fmt.Printf("%+v\n", msg)
		}
	}()

	if err := tr.ListenAndAccept(); err != nil {
		log.Fatal(err)
	}

	select {}
}
