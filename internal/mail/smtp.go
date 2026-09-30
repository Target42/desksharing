package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Config holds SMTP settings.
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
	TLSMode  string // starttls | tls | none
}

// SMTPMailer sends mail via SMTP.
type SMTPMailer struct {
	cfg Config
}

func NewSMTP(cfg Config) (*SMTPMailer, error) {
	if cfg.Host == "" || cfg.From == "" {
		return nil, fmt.Errorf("smtp host and from are required")
	}
	if cfg.Port == "" {
		cfg.Port = "587"
	}
	if cfg.TLSMode == "" {
		cfg.TLSMode = "starttls"
	}
	return &SMTPMailer{cfg: cfg}, nil
}

func (m *SMTPMailer) Send(ctx context.Context, to, subject, body string) error {
	if to == "" {
		return fmt.Errorf("mail recipient required")
	}

	fromAddr := extractAddress(m.cfg.From)
	msg := strings.Builder{}
	msg.WriteString("From: " + m.cfg.From + "\r\n")
	msg.WriteString("To: " + to + "\r\n")
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		msg.WriteString("\r\n")
	}

	addr := net.JoinHostPort(m.cfg.Host, m.cfg.Port)
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(20 * time.Second)
	}

	switch strings.ToLower(m.cfg.TLSMode) {
	case "tls", "ssl":
		return m.sendTLS(addr, fromAddr, to, []byte(msg.String()), deadline)
	case "none", "off":
		return m.sendPlain(addr, fromAddr, to, []byte(msg.String()), deadline)
	default: // starttls
		return m.sendStartTLS(addr, fromAddr, to, []byte(msg.String()), deadline)
	}
}

func (m *SMTPMailer) sendStartTLS(addr, from, to string, payload []byte, deadline time.Time) error {
	conn, err := net.DialTimeout("tcp", addr, time.Until(deadline))
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(deadline)

	host, _, _ := net.SplitHostPort(addr)
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsCfg := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
		if err := client.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}
	if err := m.auth(client); err != nil {
		return err
	}
	return m.data(client, from, to, payload)
}

func (m *SMTPMailer) sendTLS(addr, from, to string, payload []byte, deadline time.Time) error {
	host, _, _ := net.SplitHostPort(addr)
	dialer := &net.Dialer{Timeout: time.Until(deadline)}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return fmt.Errorf("smtp tls dial: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(deadline)

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if err := m.auth(client); err != nil {
		return err
	}
	return m.data(client, from, to, payload)
}

func (m *SMTPMailer) sendPlain(addr, from, to string, payload []byte, deadline time.Time) error {
	conn, err := net.DialTimeout("tcp", addr, time.Until(deadline))
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(deadline)

	host, _, _ := net.SplitHostPort(addr)
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if err := m.auth(client); err != nil {
		return err
	}
	return m.data(client, from, to, payload)
}

func (m *SMTPMailer) auth(client *smtp.Client) error {
	if m.cfg.User == "" {
		return nil
	}
	auth := smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	return nil
}

func (m *SMTPMailer) data(client *smtp.Client, from, to string, payload []byte) error {
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(payload); err != nil {
		_ = w.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}
	return client.Quit()
}

func extractAddress(from string) string {
	from = strings.TrimSpace(from)
	if i := strings.LastIndex(from, "<"); i >= 0 {
		j := strings.LastIndex(from, ">")
		if j > i {
			return strings.TrimSpace(from[i+1 : j])
		}
	}
	return from
}
