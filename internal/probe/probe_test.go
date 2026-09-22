package probe

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"obxod/internal/clienthello"
)

// The hello has to be a real record our own parser accepts, and it has to carry
// the name where a filter reads it. If our parser chokes, so will the DPI's.
func TestHelloIsARealRecordWithTheName(t *testing.T) {
	rec := hello("i.ytimg.com")

	if _, err := clienthello.Parse(rec); err != nil {
		t.Fatalf("our own hello is not a valid record: %v", err)
	}

	if !bytes.Contains(rec, []byte("i.ytimg.com")) {
		t.Error("the name is not in the hello where a filter would see it")
	}
}

// fakeConn plays back a scripted reply and can refuse to connect or reset.
type fakeConn struct {
	reply    []byte
	readErr  error
	writeErr error
	pos      int
}

func (c *fakeConn) Read(p []byte) (int, error) {
	if c.readErr != nil {
		return 0, c.readErr
	}
	if c.pos >= len(c.reply) {
		return 0, io.EOF
	}
	n := copy(p, c.reply[c.pos:])
	c.pos += n
	return n, nil
}

func (c *fakeConn) Write(p []byte) (int, error) {
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return len(p), nil
}

func (c *fakeConn) Close() error                       { return nil }
func (c *fakeConn) LocalAddr() net.Addr                { return nil }
func (c *fakeConn) RemoteAddr() net.Addr               { return nil }
func (c *fakeConn) SetDeadline(t time.Time) error      { return nil }
func (c *fakeConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *fakeConn) SetWriteDeadline(t time.Time) error { return nil }

type fakeDialer struct {
	conn    net.Conn
	dialErr error
}

func (d *fakeDialer) Dial(network, address string) (net.Conn, error) {
	return d.conn, d.dialErr
}

func TestVerdicts(t *testing.T) {
	reset := errors.New("wsarecv: An existing connection was forcibly closed by the remote host.")

	cases := map[string]struct {
		dialer *fakeDialer
		want   Verdict
	}{
		"clear": {
			&fakeDialer{conn: &fakeConn{reply: []byte{0x16}}},
			Clear,
		},
		"blocked by address": {
			&fakeDialer{dialErr: errors.New("connectex: connection timed out")},
			NoConnect,
		},
		"silence after hello": {
			&fakeDialer{conn: &fakeConn{readErr: errTimeout{}}},
			NoAnswer,
		},
		"reset on read": {
			&fakeDialer{conn: &fakeConn{readErr: reset}},
			Reset,
		},
		"reset on write": {
			&fakeDialer{conn: &fakeConn{writeErr: reset}},
			Reset,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := Host(c.dialer, "example.com", time.Second)
			if err != nil {
				t.Fatalf("Host: %v", err)
			}
			if got != c.want {
				t.Errorf("verdict = %v, want %v", got, c.want)
			}
		})
	}
}

type errTimeout struct{}

func (errTimeout) Error() string   { return "i/o timeout" }
func (errTimeout) Timeout() bool   { return true }
func (errTimeout) Temporary() bool { return true }
