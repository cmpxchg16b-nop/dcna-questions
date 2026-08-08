package msgnotify

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

// capturedEmail is one message accepted by the test SMTP server: the envelope
// sender and recipients plus the raw RFC 822 message data.
type capturedEmail struct {
	from string
	to   []string
	data []byte
}

// recordingBackend is an smtp.Backend capturing every accepted message. When
// username is non-empty, sessions support AUTH PLAIN and only accept the
// configured credentials.
type recordingBackend struct {
	mu   sync.Mutex
	msgs []capturedEmail

	username string
	password string
}

func (b *recordingBackend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &recordingSession{b: b}, nil
}

func (b *recordingBackend) captured() []capturedEmail {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]capturedEmail(nil), b.msgs...)
}

type recordingSession struct {
	b    *recordingBackend
	from string
	to   []string
}

// AuthMechanisms and Auth implement smtp.AuthSession, enabling AUTH PLAIN
// against the backend's configured credentials.

func (s *recordingSession) AuthMechanisms() []string {
	return []string{sasl.Plain}
}

func (s *recordingSession) Auth(mech string) (sasl.Server, error) {
	return sasl.NewPlainServer(func(identity, username, password string) error {
		if s.b.username == "" || username != s.b.username || password != s.b.password {
			return errors.New("invalid credentials")
		}
		return nil
	}), nil
}

func (s *recordingSession) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	return nil
}

func (s *recordingSession) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.to = append(s.to, to)
	return nil
}

func (s *recordingSession) Data(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.b.mu.Lock()
	defer s.b.mu.Unlock()
	s.b.msgs = append(s.b.msgs, capturedEmail{from: s.from, to: append([]string(nil), s.to...), data: data})
	return nil
}

func (s *recordingSession) Reset() {
	s.from = ""
	s.to = nil
}

func (s *recordingSession) Logout() error { return nil }

// startTestSMTPServer brings up an ad-hoc plaintext SMTP server backed by b,
// listening on a random loopback port, and returns its address.
func startTestSMTPServer(t *testing.T, b smtp.Backend) string {
	t.Helper()
	return startTestSMTPServerWithConfig(t, b, nil, false)
}

// startTestSMTPServerWithConfig is startTestSMTPServer with optional TLS: when
// serverTLS is non-nil the server advertises STARTTLS; when implicit is also
// true the listener itself is TLS-wrapped (smtps style).
func startTestSMTPServerWithConfig(t *testing.T, b smtp.Backend, serverTLS *tls.Config, implicit bool) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	if implicit {
		l = tls.NewListener(l, serverTLS)
	}
	s := smtp.NewServer(b)
	s.Domain = "localhost"
	s.TLSConfig = serverTLS
	s.AllowInsecureAuth = true
	s.ReadTimeout = 10 * time.Second
	s.WriteTimeout = 10 * time.Second
	go func() {
		if err := s.Serve(l); err != nil && !errors.Is(err, smtp.ErrServerClosed) {
			t.Errorf("smtp server: %v", err)
		}
	}()
	t.Cleanup(func() { s.Close() })
	return l.Addr().String()
}

// selfSignedServerTLS returns a server-side TLS config with a freshly
// generated self-signed certificate for localhost.
func selfSignedServerTLS(t *testing.T) *tls.Config {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshaling key: %v", err)
	}
	cert, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
	)
	if err != nil {
		t.Fatalf("loading key pair: %v", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}
}

// insecureTestTLS is the client-side TLS config used against the self-signed
// test server certificate.
func insecureTestTLS() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true} // test-only: server cert is self-signed
}

const (
	fakeSender    = "sender@example.com"
	fakeRecipient = "recipient@example.com"
)

func TestEmailBasedMsgSvc_RequiresServerAddr(t *testing.T) {
	if _, err := NewEmailBasedMsgSvc(EmailBasedMsgSvcInitOption{}); err == nil {
		t.Fatal("NewEmailBasedMsgSvc with empty ServerAddr succeeded, want error")
	}
}

func TestEmailBasedMsgSvc_RejectsInvalidInitOption(t *testing.T) {
	t.Run("malformed server address", func(t *testing.T) {
		if _, err := NewEmailBasedMsgSvc(EmailBasedMsgSvcInitOption{ServerAddr: "no-port"}); err == nil {
			t.Fatal("NewEmailBasedMsgSvc with malformed ServerAddr succeeded, want error")
		}
	})
	t.Run("unknown encryption", func(t *testing.T) {
		if _, err := NewEmailBasedMsgSvc(EmailBasedMsgSvcInitOption{ServerAddr: "127.0.0.1:25", Encryption: "carrier-pigeon"}); err == nil {
			t.Fatal("NewEmailBasedMsgSvc with unknown encryption succeeded, want error")
		}
	})
}

func TestEmailBasedMsgSvc_AcceptedFamilies(t *testing.T) {
	svc, err := NewEmailBasedMsgSvc(EmailBasedMsgSvcInitOption{ServerAddr: "127.0.0.1:25"})
	if err != nil {
		t.Fatalf("NewEmailBasedMsgSvc: %v", err)
	}
	for _, fams := range [][]MsgNotifyAddrFamily{
		svc.GetAcceptedSenderAddressFamilies(),
		svc.GetAcceptedRecipientAddressFamilies(),
	} {
		if len(fams) != 1 || fams[0] != MsgNotifyAddrFamilyEmail {
			t.Errorf("accepted families = %v, want [%s]", fams, MsgNotifyAddrFamilyEmail)
		}
	}
}

func TestEmailBasedMsgSvc_AreYou(t *testing.T) {
	svc, err := NewEmailBasedMsgSvc(EmailBasedMsgSvcInitOption{ServerAddr: "127.0.0.1:25"})
	if err != nil {
		t.Fatalf("NewEmailBasedMsgSvc: %v", err)
	}
	// The email service answers yes for any query.
	for _, addr := range []AddrId{
		{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeSender},
		{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout},
		{AddressFamily: MsgNotifyAddrFamilyService, Address: WellKnownAddrServiceOnMemoryExamTrackingServer},
		{},
	} {
		if !svc.AreYou(addr) {
			t.Errorf("AreYou(%v) = false, want true for any query", addr)
		}
	}
}

func TestEmailBasedMsgSvc_Send_PlainText(t *testing.T) {
	backend := &recordingBackend{}
	addr := startTestSMTPServer(t, backend)

	svc, err := NewEmailBasedMsgSvc(EmailBasedMsgSvcInitOption{ServerAddr: addr})
	if err != nil {
		t.Fatalf("NewEmailBasedMsgSvc: %v", err)
	}

	replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeSender}
	to := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeRecipient}
	msg := Msg{
		Id:      "msg-plain-1",
		Created: 1735689600000, // 2025-01-01T00:00:00Z
		Title:   "Exam session completed",
		Level:   MessageLevelCommon,
		Text:    "Congratulations, you finished the exam.",
	}
	if err := svc.Send(context.Background(), replyTo, to, msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	got := backend.captured()
	if len(got) != 1 {
		t.Fatalf("server captured %d messages, want 1", len(got))
	}
	if got[0].from != fakeSender {
		t.Errorf("envelope from = %q, want %q", got[0].from, fakeSender)
	}
	if len(got[0].to) != 1 || got[0].to[0] != fakeRecipient {
		t.Errorf("envelope to = %v, want [%s]", got[0].to, fakeRecipient)
	}

	parsed, err := mail.ReadMessage(bytes.NewReader(got[0].data))
	if err != nil {
		t.Fatalf("parsing captured message: %v", err)
	}
	if h := parsed.Header.Get("From"); h != fakeSender {
		t.Errorf("From header = %q, want %q", h, fakeSender)
	}
	if h := parsed.Header.Get("To"); h != fakeRecipient {
		t.Errorf("To header = %q, want %q", h, fakeRecipient)
	}
	if h := parsed.Header.Get("Subject"); h != msg.Title {
		t.Errorf("Subject header = %q, want %q", h, msg.Title)
	}
	if h := parsed.Header.Get("Message-Id"); h != "<"+msg.Id+">" {
		t.Errorf("Message-Id header = %q, want <%s>", h, msg.Id)
	}
	if h := parsed.Header.Get("Date"); h != time.UnixMilli(msg.Created).Format(time.RFC1123Z) {
		t.Errorf("Date header = %q, want %q", h, time.UnixMilli(msg.Created).Format(time.RFC1123Z))
	}
	body, err := io.ReadAll(parsed.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if strings.TrimSpace(string(body)) != msg.Text {
		t.Errorf("body = %q, want %q", strings.TrimSpace(string(body)), msg.Text)
	}
}

func TestEmailBasedMsgSvc_Send_WithAttachments(t *testing.T) {
	backend := &recordingBackend{}
	addr := startTestSMTPServer(t, backend)

	svc, err := NewEmailBasedMsgSvc(EmailBasedMsgSvcInitOption{ServerAddr: addr})
	if err != nil {
		t.Fatalf("NewEmailBasedMsgSvc: %v", err)
	}

	reportPDF := []byte("%PDF-1.4 fake report bytes \x00\x01\x02")
	certPNG := bytes.Repeat([]byte{0x89, 'P', 'N', 'G'}, 64) // >76 base64 chars, exercises line wrapping
	msg := Msg{
		Id:      "msg-att-1",
		Created: 1735689600000,
		Title:   "Your exam report",
		Level:   MessageLevelImportant,
		Text:    "Your report is attached.",
		Attachments: []BlobAttachment{
			{Id: "a1", Content: reportPDF, MIMEType: "application/pdf", Size: len(reportPDF), Filename: "report.pdf"},
			{Id: "a2", Content: certPNG, MIMEType: "image/png", Size: len(certPNG), Filename: "cert.png"},
		},
	}
	replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeSender}
	to := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeRecipient}
	if err := svc.Send(context.Background(), replyTo, to, msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	got := backend.captured()
	if len(got) != 1 {
		t.Fatalf("server captured %d messages, want 1", len(got))
	}

	parsed, err := mail.ReadMessage(bytes.NewReader(got[0].data))
	if err != nil {
		t.Fatalf("parsing captured message: %v", err)
	}
	mediaType, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parsing Content-Type: %v", err)
	}
	if mediaType != "multipart/mixed" {
		t.Fatalf("Content-Type = %q, want multipart/mixed", mediaType)
	}

	mr := multipart.NewReader(parsed.Body, params["boundary"])

	// First part: the plaintext body.
	part, err := mr.NextPart()
	if err != nil {
		t.Fatalf("reading text part: %v", err)
	}
	body, err := io.ReadAll(part)
	if err != nil {
		t.Fatalf("reading text part body: %v", err)
	}
	if string(body) != msg.Text {
		t.Errorf("text part = %q, want %q", string(body), msg.Text)
	}

	// Remaining parts: the attachments, in order.
	for i, want := range msg.Attachments {
		part, err := mr.NextPart()
		if err != nil {
			t.Fatalf("reading attachment part %d: %v", i, err)
		}
		if fn := part.FileName(); fn != want.Filename {
			t.Errorf("attachment %d filename = %q, want %q", i, fn, want.Filename)
		}
		if ct := part.Header.Get("Content-Type"); !strings.HasPrefix(ct, want.MIMEType) {
			t.Errorf("attachment %d Content-Type = %q, want prefix %q", i, ct, want.MIMEType)
		}
		decoded, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, part))
		if err != nil {
			t.Fatalf("decoding attachment %d: %v", i, err)
		}
		if !bytes.Equal(decoded, want.Content) {
			t.Errorf("attachment %d content = %d bytes, does not round-trip the original %d bytes", i, len(decoded), len(want.Content))
		}
	}
	if _, err := mr.NextPart(); err != io.EOF {
		t.Errorf("extra parts after attachments, NextPart = %v, want io.EOF", err)
	}
}

// sendAndCapture delivers msg through an EmailBasedMsgSvc pointed at a
// recording test server and returns the parsed captured message.
func sendAndCapture(t *testing.T, msg Msg) *mail.Message {
	t.Helper()
	backend := &recordingBackend{}
	addr := startTestSMTPServer(t, backend)

	svc, err := NewEmailBasedMsgSvc(EmailBasedMsgSvcInitOption{ServerAddr: addr})
	if err != nil {
		t.Fatalf("NewEmailBasedMsgSvc: %v", err)
	}
	replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeSender}
	to := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeRecipient}
	if err := svc.Send(context.Background(), replyTo, to, msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	got := backend.captured()
	if len(got) != 1 {
		t.Fatalf("server captured %d messages, want 1", len(got))
	}
	parsed, err := mail.ReadMessage(bytes.NewReader(got[0].data))
	if err != nil {
		t.Fatalf("parsing captured message: %v", err)
	}
	return parsed
}

// mimePart is a fully read MIME part: headers plus the raw (still
// transfer-encoded) body. Bodies must be captured eagerly: multipart.Part
// streams over the underlying reader, so a part's body is gone once the
// next part is started.
type mimePart struct {
	header   textproto.MIMEHeader
	filename string
	body     []byte
}

// readMultipart returns all parts of a multipart body in order.
func readMultipart(t *testing.T, body io.Reader, contentType string) []mimePart {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parsing Content-Type %q: %v", contentType, err)
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		t.Fatalf("Content-Type = %q, want a multipart media type", mediaType)
	}
	mr := multipart.NewReader(body, params["boundary"])
	var parts []mimePart
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return parts
		}
		if err != nil {
			t.Fatalf("reading part %d: %v", len(parts), err)
		}
		raw, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("reading part %d body: %v", len(parts), err)
		}
		parts = append(parts, mimePart{header: part.Header, filename: part.FileName(), body: raw})
	}
}

func TestEmailBasedMsgSvc_Send_HTML(t *testing.T) {
	msg := Msg{
		Id:      "msg-html-1",
		Created: 1735689600000,
		Title:   "Exam session completed",
		Level:   MessageLevelCommon,
		Text:    "Congratulations, you finished the exam.",
		HTML:    "<html><body><h1>Congratulations</h1><p>you finished the exam.</p></body></html>",
	}
	parsed := sendAndCapture(t, msg)

	mediaType, _, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parsing Content-Type: %v", err)
	}
	if mediaType != "multipart/alternative" {
		t.Fatalf("Content-Type = %q, want multipart/alternative", mediaType)
	}

	parts := readMultipart(t, parsed.Body, parsed.Header.Get("Content-Type"))
	if len(parts) != 2 {
		t.Fatalf("got %d parts, want 2 (text then html)", len(parts))
	}
	// The plaintext part comes first so HTML-incapable clients can fall back.
	if ct := parts[0].header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("part 0 Content-Type = %q, want text/plain", ct)
	}
	if body := string(parts[0].body); body != msg.Text {
		t.Errorf("text part = %q, want %q", body, msg.Text)
	}
	if ct := parts[1].header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("part 1 Content-Type = %q, want text/html", ct)
	}
	if body := string(parts[1].body); body != msg.HTML {
		t.Errorf("html part = %q, want %q", body, msg.HTML)
	}
}

func TestEmailBasedMsgSvc_Send_HTMLWithAttachment(t *testing.T) {
	reportXML := []byte("<?xml version=\"1.0\"?><examreport id=\"r1\"></examreport>")
	msg := Msg{
		Id:      "msg-html-att-1",
		Created: 1735689600000,
		Title:   "Your exam report",
		Level:   MessageLevelCommon,
		Text:    "Your report is attached.",
		HTML:    "<html><body><p>Your report is <strong>attached</strong>.</p></body></html>",
		Attachments: []BlobAttachment{
			{Id: "a1", Content: reportXML, MIMEType: "application/xml", Size: len(reportXML), Filename: "exam-report-r1.xml"},
		},
	}
	parsed := sendAndCapture(t, msg)

	mediaType, _, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parsing Content-Type: %v", err)
	}
	if mediaType != "multipart/mixed" {
		t.Fatalf("Content-Type = %q, want multipart/mixed", mediaType)
	}

	parts := readMultipart(t, parsed.Body, parsed.Header.Get("Content-Type"))
	if len(parts) != 2 {
		t.Fatalf("got %d parts, want 2 (body then attachment)", len(parts))
	}

	// The body part is a nested multipart/alternative of text and html.
	bodyParts := readMultipart(t, bytes.NewReader(parts[0].body), parts[0].header.Get("Content-Type"))
	if len(bodyParts) != 2 {
		t.Fatalf("body part has %d sub-parts, want 2 (text then html)", len(bodyParts))
	}
	if ct := bodyParts[0].header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("body sub-part 0 Content-Type = %q, want text/plain", ct)
	}
	if body := string(bodyParts[0].body); body != msg.Text {
		t.Errorf("text sub-part = %q, want %q", body, msg.Text)
	}
	if ct := bodyParts[1].header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("body sub-part 1 Content-Type = %q, want text/html", ct)
	}
	if body := string(bodyParts[1].body); body != msg.HTML {
		t.Errorf("html sub-part = %q, want %q", body, msg.HTML)
	}

	// The attachment follows, base64-encoded as before.
	attachment := parts[1]
	if attachment.filename != "exam-report-r1.xml" {
		t.Errorf("attachment filename = %q, want exam-report-r1.xml", attachment.filename)
	}
	decoded, err := base64.StdEncoding.DecodeString(string(attachment.body))
	if err != nil {
		t.Fatalf("decoding attachment: %v", err)
	}
	if !bytes.Equal(decoded, reportXML) {
		t.Errorf("attachment content does not round-trip: got %q", decoded)
	}
}

func TestEmailBasedMsgSvc_Send_WithAuth(t *testing.T) {
	backend := &recordingBackend{username: fakeSender, password: "s3cret"}
	addr := startTestSMTPServer(t, backend)

	replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeSender}
	to := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeRecipient}
	msg := Msg{Id: "msg-auth-1", Created: 1735689600000, Title: "t", Level: MessageLevelCommon, Text: "authenticated"}

	t.Run("valid credentials succeed", func(t *testing.T) {
		svc, err := NewEmailBasedMsgSvc(EmailBasedMsgSvcInitOption{
			ServerAddr: addr,
			Username:   fakeSender,
			Password:   "s3cret",
		})
		if err != nil {
			t.Fatalf("NewEmailBasedMsgSvc: %v", err)
		}
		if err := svc.Send(context.Background(), replyTo, to, msg); err != nil {
			t.Fatalf("Send: %v", err)
		}
		if got := backend.captured(); len(got) != 1 {
			t.Fatalf("server captured %d messages, want 1", len(got))
		}
	})

	t.Run("invalid credentials fail", func(t *testing.T) {
		svc, err := NewEmailBasedMsgSvc(EmailBasedMsgSvcInitOption{
			ServerAddr: addr,
			Username:   fakeSender,
			Password:   "wrong",
		})
		if err != nil {
			t.Fatalf("NewEmailBasedMsgSvc: %v", err)
		}
		if err := svc.Send(context.Background(), replyTo, to, msg); err == nil {
			t.Fatal("Send with wrong password succeeded, want error")
		}
	})
}

func TestEmailBasedMsgSvc_RejectsNonEmailFamilies(t *testing.T) {
	backend := &recordingBackend{}
	addr := startTestSMTPServer(t, backend)

	svc, err := NewEmailBasedMsgSvc(EmailBasedMsgSvcInitOption{ServerAddr: addr})
	if err != nil {
		t.Fatalf("NewEmailBasedMsgSvc: %v", err)
	}
	msg := Msg{Id: "msg-rej-1", Created: 1735689600000, Title: "t", Level: MessageLevelCommon, Text: "nope"}

	t.Run("non-email recipient", func(t *testing.T) {
		replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeSender}
		to := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout}
		err := svc.Send(context.Background(), replyTo, to, msg)
		if err == nil {
			t.Fatal("Send succeeded, want error")
		}
		if want := `unsupported recipient address family "console"`; !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err, want)
		}
	})

	t.Run("non-email sender", func(t *testing.T) {
		replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyService, Address: "exam-server"}
		to := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeRecipient}
		err := svc.Send(context.Background(), replyTo, to, msg)
		if err == nil {
			t.Fatal("Send succeeded, want error")
		}
		if want := `unsupported sender address family "service"`; !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err, want)
		}
	})

	if got := backend.captured(); len(got) != 0 {
		t.Errorf("server captured %d messages, want 0", len(got))
	}
}

func TestEmailBasedMsgSvc_Send_StartTLS(t *testing.T) {
	backend := &recordingBackend{}
	addr := startTestSMTPServerWithConfig(t, backend, selfSignedServerTLS(t), false)

	svc, err := NewEmailBasedMsgSvc(EmailBasedMsgSvcInitOption{
		ServerAddr: addr,
		Encryption: SMTPEncryptionStartTLS,
		TLSConfig:  insecureTestTLS(),
	})
	if err != nil {
		t.Fatalf("NewEmailBasedMsgSvc: %v", err)
	}

	replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeSender}
	to := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeRecipient}
	msg := Msg{Id: "msg-starttls-1", Created: 1735689600000, Title: "t", Level: MessageLevelCommon, Text: "over starttls"}
	if err := svc.Send(context.Background(), replyTo, to, msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got := backend.captured(); len(got) != 1 {
		t.Fatalf("server captured %d messages, want 1", len(got))
	}
}

func TestEmailBasedMsgSvc_StartTLSNeverFallsBackToPlaintext(t *testing.T) {
	// The server has no TLS configured, so it does not advertise STARTTLS.
	backend := &recordingBackend{}
	addr := startTestSMTPServer(t, backend)

	// Credentials are configured to prove they are never sent: the client
	// must fail at STARTTLS, before any AUTH exchange.
	svc, err := NewEmailBasedMsgSvc(EmailBasedMsgSvcInitOption{
		ServerAddr: addr,
		Encryption: SMTPEncryptionStartTLS,
		TLSConfig:  insecureTestTLS(),
		Username:   fakeSender,
		Password:   "s3cret",
	})
	if err != nil {
		t.Fatalf("NewEmailBasedMsgSvc: %v", err)
	}

	replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeSender}
	to := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeRecipient}
	msg := Msg{Id: "msg-nofallback-1", Created: 1735689600000, Title: "t", Level: MessageLevelImportant, Text: "must not be sent"}

	err = svc.Send(context.Background(), replyTo, to, msg)
	if err == nil {
		t.Fatal("Send to a plaintext-only server succeeded, want STARTTLS failure")
	}
	if !strings.Contains(err.Error(), "STARTTLS") {
		t.Errorf("error = %q, want it to mention STARTTLS", err)
	}
	if got := backend.captured(); len(got) != 0 {
		t.Errorf("server captured %d messages, want 0: nothing may be sent insecurely", len(got))
	}
}

func TestEmailBasedMsgSvc_Send_ImplicitTLS(t *testing.T) {
	backend := &recordingBackend{}
	addr := startTestSMTPServerWithConfig(t, backend, selfSignedServerTLS(t), true)

	svc, err := NewEmailBasedMsgSvc(EmailBasedMsgSvcInitOption{
		ServerAddr: addr,
		Encryption: SMTPEncryptionTLS,
		TLSConfig:  insecureTestTLS(),
	})
	if err != nil {
		t.Fatalf("NewEmailBasedMsgSvc: %v", err)
	}

	replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeSender}
	to := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeRecipient}
	msg := Msg{Id: "msg-tls-1", Created: 1735689600000, Title: "t", Level: MessageLevelCommon, Text: "over implicit tls"}
	if err := svc.Send(context.Background(), replyTo, to, msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got := backend.captured(); len(got) != 1 {
		t.Fatalf("server captured %d messages, want 1", len(got))
	}
}

func TestEmailBasedMsgSvc_ImplicitTLSRejectsPlaintextServer(t *testing.T) {
	backend := &recordingBackend{}
	addr := startTestSMTPServer(t, backend)

	svc, err := NewEmailBasedMsgSvc(EmailBasedMsgSvcInitOption{
		ServerAddr: addr,
		Encryption: SMTPEncryptionTLS,
		TLSConfig:  insecureTestTLS(),
		Username:   fakeSender,
		Password:   "s3cret",
	})
	if err != nil {
		t.Fatalf("NewEmailBasedMsgSvc: %v", err)
	}

	replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeSender}
	to := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeRecipient}
	msg := Msg{Id: "msg-tlsreject-1", Created: 1735689600000, Title: "t", Level: MessageLevelImportant, Text: "must not be sent"}

	if err := svc.Send(context.Background(), replyTo, to, msg); err == nil {
		t.Fatal("Send to a plaintext server succeeded, want TLS handshake failure")
	}
	if got := backend.captured(); len(got) != 0 {
		t.Errorf("server captured %d messages, want 0: nothing may be sent insecurely", len(got))
	}
}

func TestEmailBasedMsgSvc_ConcurrentSends(t *testing.T) {
	backend := &recordingBackend{}
	addr := startTestSMTPServer(t, backend)

	svc, err := NewEmailBasedMsgSvc(EmailBasedMsgSvcInitOption{ServerAddr: addr})
	if err != nil {
		t.Fatalf("NewEmailBasedMsgSvc: %v", err)
	}

	replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeSender}
	to := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: fakeRecipient}

	const senders = 16
	var wg sync.WaitGroup
	errs := make(chan error, senders)
	for i := 0; i < senders; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			msg := Msg{
				Id:      "msg-conc-" + strings.Repeat("x", i+1),
				Created: 1735689600000,
				Title:   "concurrent",
				Level:   MessageLevelCommon,
				Text:    "simultaneous send",
			}
			if err := svc.Send(context.Background(), replyTo, to, msg); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent Send: %v", err)
	}
	if got := backend.captured(); len(got) != senders {
		t.Errorf("server captured %d messages, want %d", len(got), senders)
	}
}
