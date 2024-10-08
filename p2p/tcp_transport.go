package p2p

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
)

type TCPPeer struct {
	net.Conn
	outbound bool
	wg       *sync.WaitGroup
}

func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		Conn:     conn,
		outbound: outbound,
		wg:       &sync.WaitGroup{},
	}
}

func (p *TCPPeer) CloseStream() {
	p.wg.Done()
}

// Send implements the Peer interface
func (p *TCPPeer) Send(b []byte) error {
	_, err := p.Conn.Write(b)

	return err
}

type TCPTransferOpts struct {
	ListenAddress string
	Listener      net.Listener
	ShakeHands    HandshakeFunc
	Decoder       Decoder
	OnPeer        func(Peer) error
}

type TCPTransport struct {
	TCPTransferOpts
	rpcCh chan RPC
}

func NewTCPTransport(opts TCPTransferOpts) *TCPTransport {
	return &TCPTransport{
		TCPTransferOpts: opts,
		rpcCh:           make(chan RPC),
	}
}

// Addr implements the transport interface and returns the listening address
// transport is accepting on.
func (t *TCPTransport) Addr() string {
	return t.ListenAddress
}

// ListenAndAccept implements the Transport interface
func (t *TCPTransport) ListenAndAccept() error {
	var err error
	t.Listener, err = net.Listen("tcp", t.ListenAddress)

	if err != nil {
		return err
	}
	go t.startAcceptLoop()
	log.Printf("TCP transport listening on port %s\n", t.ListenAddress)
	return nil
}

// Consume implements the Transport interface, which will return read-only channel
// for reading the incoming messages received from another peer in the
func (t *TCPTransport) Consume() <-chan RPC {
	return t.rpcCh
}

// Close implements the Transport interface
func (t *TCPTransport) Close() error {
	return t.Listener.Close()
}

// Dial implements the Transport interface which dials to a remote addr of type string
func (t *TCPTransport) Dial(addr string) error {
	conn, err := net.Dial("tcp", addr)

	if err != nil {
		return err
	}
	go t.handleConn(conn, true)
	return nil
}

func (t *TCPTransport) startAcceptLoop() {
	for {
		conn, err := t.Listener.Accept()

		if errors.Is(err, net.ErrClosed) {
			return
		}

		if err != nil {
			fmt.Printf("TCP accept error: %s\n", err)
		}
		fmt.Printf("New incoming connection on server %s: %+v\n", conn.LocalAddr(), conn.RemoteAddr())

		go t.handleConn(conn, false)

	}
}

func (t *TCPTransport) handleConn(conn net.Conn, outbound bool) {
	var err error

	defer func() {
		fmt.Printf("Dropping peer connection: %s\n", err)
		conn.Close()
	}()

	peer := NewTCPPeer(conn, outbound)

	if err = t.ShakeHands(peer); err != nil {
		return
	}

	if t.OnPeer != nil {
		if err = t.OnPeer(peer); err != nil {
			return
		}
	}

	// Read loop
	for {
		rpc := RPC{}
		err = t.Decoder.Decode(conn, &rpc)
		if err != nil {
			fmt.Printf("TCP decoding error: %v\n", err)
		}

		rpc.From = conn.RemoteAddr().String()

		if rpc.Stream {
			peer.wg.Add(1)
			fmt.Printf("[%s] incoming stream, waiting...\n", conn.RemoteAddr())
			peer.wg.Wait()
			fmt.Printf("[%s] stream close, resuming read loop\n", conn.RemoteAddr())
			continue
		}
		t.rpcCh <- rpc
	}
}
