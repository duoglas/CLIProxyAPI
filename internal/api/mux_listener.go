package api

import (
	"errors"
	"net"
	"sync"
)

var errMuxListenerFull = errors.New("mux listener buffer full")

type muxListener struct {
	addr    net.Addr
	connCh  chan net.Conn
	closeCh chan struct{}
	once    sync.Once
	mu      sync.Mutex
	closed  bool
}

func newMuxListener(addr net.Addr, buffer int) *muxListener {
	if buffer <= 0 {
		buffer = 1
	}
	return &muxListener{
		addr:    addr,
		connCh:  make(chan net.Conn, buffer),
		closeCh: make(chan struct{}),
	}
}

func (l *muxListener) Put(conn net.Conn) error {
	if l == nil {
		return net.ErrClosed
	}
	if conn == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return net.ErrClosed
	}
	select {
	case l.connCh <- conn:
		return nil
	default:
		return errMuxListenerFull
	}
}

func (l *muxListener) Accept() (net.Conn, error) {
	if l == nil {
		return nil, net.ErrClosed
	}
	select {
	case <-l.closeCh:
		return nil, net.ErrClosed
	case conn := <-l.connCh:
		if conn == nil {
			return nil, net.ErrClosed
		}
		return conn, nil
	}
}

func (l *muxListener) Close() error {
	if l == nil {
		return nil
	}
	l.once.Do(func() {
		l.mu.Lock()
		l.closed = true
		l.mu.Unlock()
		close(l.closeCh)
	})
	return nil
}

func (l *muxListener) Addr() net.Addr {
	if l == nil {
		return &net.TCPAddr{}
	}
	if l.addr == nil {
		return &net.TCPAddr{}
	}
	return l.addr
}
