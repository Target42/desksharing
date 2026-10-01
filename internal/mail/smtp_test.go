package mail

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestStartTLSFailsClosed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		r := bufio.NewReader(conn)
		_, _ = conn.Write([]byte("220 localhost ESMTP\r\n"))
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			upper := strings.ToUpper(line)
			switch {
			case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
				_, _ = conn.Write([]byte("250-localhost\r\n250 HELP\r\n"))
			case strings.HasPrefix(upper, "QUIT"):
				_, _ = conn.Write([]byte("221 bye\r\n"))
				return
			default:
				_, _ = conn.Write([]byte("250 OK\r\n"))
			}
		}
	}()

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	m, err := NewSMTP(Config{Host: host, Port: port, From: "Desk <noreply@example.com>", TLSMode: "starttls"})
	if err != nil {
		t.Fatal(err)
	}
	err = m.Send(context.Background(), "person@example.com", "Betreff", "Hallo")
	if err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("got %v", err)
	}
	_ = ln.Close()
	<-done
}

func TestSendRejectsHeaderInjection(t *testing.T) {
	m, err := NewSMTP(Config{Host: "127.0.0.1", Port: "2525", From: "Desk <noreply@example.com>"})
	if err != nil {
		t.Fatal(err)
	}
	err = m.Send(context.Background(), "a@b.de\r\nBcc: x@y.z", "Hi", "body")
	if err == nil || !strings.Contains(err.Error(), "line break") {
		t.Fatalf("got %v", err)
	}
}
