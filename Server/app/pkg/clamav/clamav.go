// Package clamav scans a stream with a clamd daemon over TCP (the INSTREAM
// command) — used by catalog-service to virus-scan e-library uploads
// before they're stored. Protocol: https://docs.clamav.net/manual/Usage/Scanning.html#clamd
package clamav

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// ErrInfected carries the signature name clamd reported.
type ErrInfected struct{ Signature string }

func (e *ErrInfected) Error() string { return "infected: " + e.Signature }

type Client struct {
	addr    string
	timeout time.Duration
}

func New(addr string) *Client {
	return &Client{addr: addr, timeout: 2 * time.Minute}
}

// Scan streams r to clamd. nil = clean; *ErrInfected = malware found; any
// other error = the scan couldn't be completed (callers fail closed).
func (c *Client) Scan(ctx context.Context, r io.Reader) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return fmt.Errorf("clamd unreachable: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(c.timeout))

	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return err
	}
	buf := make([]byte, 32*1024)
	var size [4]byte
	for {
		n, rerr := r.Read(buf)
		if n > 0 {
			binary.BigEndian.PutUint32(size[:], uint32(n))
			if _, err := conn.Write(size[:]); err != nil {
				return err
			}
			if _, err := conn.Write(buf[:n]); err != nil {
				return err
			}
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	binary.BigEndian.PutUint32(size[:], 0) // end of stream
	if _, err := conn.Write(size[:]); err != nil {
		return err
	}

	reply, err := bufio.NewReader(conn).ReadString(0)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	reply = strings.TrimRight(reply, "\x00\n ")
	switch {
	case strings.HasSuffix(reply, " OK"):
		return nil
	case strings.HasSuffix(reply, " FOUND"):
		sig := strings.TrimSuffix(strings.TrimPrefix(reply, "stream: "), " FOUND")
		return &ErrInfected{Signature: sig}
	default:
		return fmt.Errorf("clamd: %s", reply)
	}
}

// Ping checks clamd answers — for /health.
func (c *Client) Ping(ctx context.Context) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte("zPING\x00")); err != nil {
		return err
	}
	reply, err := bufio.NewReader(conn).ReadString(0)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if strings.TrimRight(reply, "\x00") != "PONG" {
		return fmt.Errorf("clamd: unexpected ping reply %q", reply)
	}
	return nil
}
