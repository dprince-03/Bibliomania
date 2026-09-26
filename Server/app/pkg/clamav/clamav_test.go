package clamav

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
)

// fakeClamd answers INSTREAM like clamd: FOUND if the stream contains the
// EICAR test string, OK otherwise.
func fakeClamd(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				cmd, _ := r.ReadString(0)
				if cmd == "zPING\x00" {
					c.Write([]byte("PONG\x00"))
					return
				}
				var data []byte
				for {
					var size [4]byte
					if _, err := io.ReadFull(r, size[:]); err != nil {
						return
					}
					n := binary.BigEndian.Uint32(size[:])
					if n == 0 {
						break
					}
					chunk := make([]byte, n)
					io.ReadFull(r, chunk)
					data = append(data, chunk...)
				}
				if strings.Contains(string(data), "EICAR-STANDARD-ANTIVIRUS-TEST-FILE") {
					c.Write([]byte("stream: Eicar-Test-Signature FOUND\x00"))
				} else {
					c.Write([]byte("stream: OK\x00"))
				}
			}(conn)
		}
	}()
	return ln.Addr().String()
}

func TestScan(t *testing.T) {
	c := New(fakeClamd(t))
	ctx := context.Background()
	if err := c.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	if err := c.Scan(ctx, strings.NewReader(strings.Repeat("%PDF-1.7 clean ", 10000))); err != nil {
		t.Fatalf("clean file: %v", err)
	}
	err := c.Scan(ctx, strings.NewReader(`X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`))
	var infected *ErrInfected
	if !errors.As(err, &infected) || infected.Signature != "Eicar-Test-Signature" {
		t.Fatalf("eicar: %v", err)
	}
	if err := New("127.0.0.1:1").Scan(ctx, strings.NewReader("x")); err == nil || errors.As(err, &infected) {
		t.Fatalf("unreachable clamd should be a plain error, got %v", err)
	}
}
