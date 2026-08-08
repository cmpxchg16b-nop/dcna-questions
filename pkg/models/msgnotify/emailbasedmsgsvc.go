package msgnotify

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"time"
)

// SMTPEncryption selects how the connection to the SMTP server is secured.
type SMTPEncryption string

const (
	// SMTPEncryptionNone uses a plaintext connection. It offers no transport
	// security and should only be used on trusted networks.
	SMTPEncryptionNone SMTPEncryption = "none"

	// SMTPEncryptionStartTLS connects in plaintext and then upgrades the
	// connection with the STARTTLS command before authenticating or sending
	// any message data. The upgrade is mandatory: if the server does not
	// support STARTTLS, Send fails instead of falling back to an insecure
	// connection.
	SMTPEncryptionStartTLS SMTPEncryption = "starttls"

	// SMTPEncryptionTLS establishes TLS immediately upon connecting (the
	// "smtps" style, typically port 465). No plaintext is ever exchanged;
	// insecure transport is not possible.
	SMTPEncryptionTLS SMTPEncryption = "tls"
)

// EmailBasedMsgSvcInitOption carries the parameters needed to construct an
// EmailBasedMsgSvc.
type EmailBasedMsgSvcInitOption struct {
	// ServerAddr is the SMTP server address in "host:port" form.
	ServerAddr string

	// Encryption selects the transport security. The empty value defaults to
	// SMTPEncryptionNone.
	Encryption SMTPEncryption

	// TLSConfig is the TLS client configuration used with
	// SMTPEncryptionStartTLS and SMTPEncryptionTLS. When nil, a default
	// config with the server host as ServerName and a minimum version of TLS
	// 1.2 is used. It is cloned, so later mutation by the caller has no
	// effect.
	TLSConfig *tls.Config

	// Username and Password are the SMTP AUTH PLAIN credentials. When
	// Username is empty, no authentication is attempted.
	Username string
	Password string
}

// EmailBasedMsgSvc is a MsgNotifySvc that delivers messages as emails through
// an SMTP server. It is stateless — every Send opens a fresh SMTP connection
// and keeps no per-call state — so a single instance is safe for concurrent
// use by multiple goroutines without explicit coordination.
type EmailBasedMsgSvc struct {
	opt       EmailBasedMsgSvcInitOption
	host      string
	tlsConfig *tls.Config
}

var _ MsgNotifySvc = (*EmailBasedMsgSvc)(nil)

// NewEmailBasedMsgSvc returns an EmailBasedMsgSvc configured by opt.
func NewEmailBasedMsgSvc(opt EmailBasedMsgSvcInitOption) (*EmailBasedMsgSvc, error) {
	if opt.ServerAddr == "" {
		return nil, errors.New("msgnotify: email service requires an SMTP server address")
	}
	host, _, err := net.SplitHostPort(opt.ServerAddr)
	if err != nil {
		return nil, fmt.Errorf("msgnotify: invalid SMTP server address %q: %w", opt.ServerAddr, err)
	}

	switch opt.Encryption {
	case "":
		opt.Encryption = SMTPEncryptionNone
	case SMTPEncryptionNone, SMTPEncryptionStartTLS, SMTPEncryptionTLS:
	default:
		return nil, fmt.Errorf("msgnotify: unsupported SMTP encryption %q", opt.Encryption)
	}

	tlsConfig := opt.TLSConfig
	if tlsConfig != nil {
		tlsConfig = tlsConfig.Clone()
		if tlsConfig.ServerName == "" {
			tlsConfig.ServerName = host
		}
	} else {
		tlsConfig = &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	}

	return &EmailBasedMsgSvc{opt: opt, host: host, tlsConfig: tlsConfig}, nil
}

// AreYou always answers yes: the email service claims any address it is
// asked about.
func (s *EmailBasedMsgSvc) AreYou(addrId AddrId) bool {
	return true
}

// GetAcceptedSenderAddressFamilies returns only the email family: the sender
// address becomes the email's From address.
func (s *EmailBasedMsgSvc) GetAcceptedSenderAddressFamilies() []MsgNotifyAddrFamily {
	return []MsgNotifyAddrFamily{MsgNotifyAddrFamilyEmail}
}

// GetAcceptedRecipientAddressFamilies returns only the email family.
func (s *EmailBasedMsgSvc) GetAcceptedRecipientAddressFamilies() []MsgNotifyAddrFamily {
	return []MsgNotifyAddrFamily{MsgNotifyAddrFamilyEmail}
}

// Send delivers msg as an email to to.Address, with replyTo.Address as the
// sender (From) address. Attachments are delivered as MIME parts of a
// multipart/mixed message.
func (s *EmailBasedMsgSvc) Send(ctx context.Context, replyTo AddrId, to AddrId, msg Msg) error {
	if replyTo.AddressFamily != MsgNotifyAddrFamilyEmail {
		return fmt.Errorf("msgnotify: unsupported sender address family %q", replyTo.AddressFamily)
	}
	if to.AddressFamily != MsgNotifyAddrFamilyEmail {
		return fmt.Errorf("msgnotify: unsupported recipient address family %q", to.AddressFamily)
	}

	raw, err := buildEmail(replyTo.Address, to.Address, msg)
	if err != nil {
		return err
	}

	client, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	if s.opt.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", s.opt.Username, s.opt.Password, s.host)); err != nil {
			return fmt.Errorf("msgnotify: SMTP authentication: %w", err)
		}
	}
	if err := client.Mail(replyTo.Address); err != nil {
		return fmt.Errorf("msgnotify: SMTP MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("msgnotify: SMTP RCPT TO: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("msgnotify: SMTP DATA: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		return fmt.Errorf("msgnotify: writing message data: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("msgnotify: finalizing message data: %w", err)
	}
	return client.Quit()
}

// connect dials the SMTP server and returns a client whose connection already
// satisfies the configured encryption: TLS is established before this returns
// (implicitly, or via a mandatory STARTTLS upgrade), so no credentials or
// message data can ever be sent over an insecure connection.
func (s *EmailBasedMsgSvc) connect(ctx context.Context) (*smtp.Client, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", s.opt.ServerAddr)
	if err != nil {
		return nil, fmt.Errorf("msgnotify: connecting to SMTP server: %w", err)
	}

	if s.opt.Encryption == SMTPEncryptionTLS {
		// TLS from the first byte: the handshake happens before any SMTP
		// traffic, so insecure transport is impossible.
		tlsConn := tls.Client(conn, s.tlsConfig)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close()
			return nil, fmt.Errorf("msgnotify: TLS handshake with SMTP server: %w", err)
		}
		conn = tlsConn
	}

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("msgnotify: initiating SMTP session: %w", err)
	}

	if s.opt.Encryption == SMTPEncryptionStartTLS {
		// StartTLS issues EHLO first and errors out when the server does
		// not advertise STARTTLS, so we never fall back to plaintext.
		if err := client.StartTLS(s.tlsConfig); err != nil {
			client.Close()
			return nil, fmt.Errorf("msgnotify: STARTTLS: %w", err)
		}
	}
	return client, nil
}

// buildEmail renders msg as an RFC 822 message with the given envelope
// addresses. When msg carries no attachments it is a simple text/plain
// message; otherwise it is multipart/mixed with the plaintext body as the
// first part followed by one base64-encoded part per attachment.
func buildEmail(from, to string, msg Msg) ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", msg.Title)
	fmt.Fprintf(&b, "Date: %s\r\n", time.UnixMilli(msg.Created).Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-Id: <%s>\r\n", msg.Id)
	b.WriteString("MIME-Version: 1.0\r\n")

	if len(msg.Attachments) == 0 {
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		b.WriteString("\r\n")
		b.WriteString(msg.Text)
		b.WriteString("\r\n")
		return b.Bytes(), nil
	}

	mw := multipart.NewWriter(&b)
	fmt.Fprintf(&b, "Content-Type: multipart/mixed; boundary=%q\r\n", mw.Boundary())
	b.WriteString("\r\n")

	textHeader := textproto.MIMEHeader{}
	textHeader.Set("Content-Type", "text/plain; charset=UTF-8")
	pw, err := mw.CreatePart(textHeader)
	if err != nil {
		return nil, err
	}
	if _, err := pw.Write([]byte(msg.Text)); err != nil {
		return nil, err
	}

	for _, a := range msg.Attachments {
		h := textproto.MIMEHeader{}
		h.Set("Content-Type", fmt.Sprintf("%s; name=%q", a.MIMEType, a.Filename))
		h.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", a.Filename))
		h.Set("Content-Transfer-Encoding", "base64")
		pw, err := mw.CreatePart(h)
		if err != nil {
			return nil, err
		}
		// RFC 2045 requires base64 lines to be at most 76 characters long.
		enc := base64.StdEncoding.EncodeToString(a.Content)
		for len(enc) > 76 {
			if _, err := pw.Write([]byte(enc[:76] + "\r\n")); err != nil {
				return nil, err
			}
			enc = enc[76:]
		}
		if _, err := pw.Write([]byte(enc + "\r\n")); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
