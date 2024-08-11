package p2p

type HandshakeFunc func(Peer) error

func DefHandshakeFunc(Peer) error { return nil }
