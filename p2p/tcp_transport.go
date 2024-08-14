package p2p

import (
	"errors"
	"fmt"
	"log"
	"net"
	"time"
)

type TCPPeer struct {
	conn     net.Conn
	outbound bool
}

func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		conn:     conn,
		outbound: outbound,
	}
}

// Send implements the Peer interface
func (p *TCPPeer) Send(b []byte) error {
	_, err := p.conn.Write(b)

	return err
}

// RemoteAddr implements the Peer interface
func (p *TCPPeer) RemoteAddr() net.Addr {
	return p.conn.RemoteAddr()
}

// Close implements the Peer interface
func (p *TCPPeer) Close() error {
	return p.conn.Close()
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
	log.Println(conn)
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
		fmt.Printf("New incoming connection: %+v\n", conn.RemoteAddr())

		go t.handleConn(conn)

	}
}

func (t *TCPTransport) handleConn(conn net.Conn) {
	var err error

	defer func() {
		fmt.Printf("Dropping peer connection: %s\n", err)
		conn.Close()
	}()

	peer := NewTCPPeer(conn, true)

	if err = t.ShakeHands(peer); err != nil {
		return
	}

	if t.OnPeer != nil {
		if err = t.OnPeer(peer); err != nil {
			return
		}
	}

	// Read loop
	rpc := RPC{}
	errCount := 0
	for {
		err = t.Decoder.Decode(conn, &rpc)
		if err != nil {
			time.Sleep(1 * time.Second)
			fmt.Printf("TCP decoding error: %v\n", err)
			errCount += 1
			if errCount > 5 {
				fmt.Println("Error decoding messages")
				return
			}
			continue
		}

		rpc.From = conn.RemoteAddr()
		t.rpcCh <- rpc
	}
}
