package p2p

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTCPTansport(t *testing.T) {
	listenAddress := ":4000"
	opts := TCPTransferOpts{
		ListenAddress: listenAddress,
		ShakeHands:    DefHandshakeFunc,
		Decoder:       GOBDecoder{},
	}
	tr := NewTCPTransport(opts)

	assert.Equal(t, listenAddress, tr.ListenAddress)

	assert.Nil(t, tr.ListenAndAccept())
}
