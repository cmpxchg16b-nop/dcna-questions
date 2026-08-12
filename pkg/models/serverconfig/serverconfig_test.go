package serverconfig

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeTestKeyPair generates a self-signed RSA certificate and writes the
// certificate and private key as PEM files into dir, returning their paths.
func writeTestKeyPair(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "serverconfig test"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("WriteFile cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("WriteFile key: %v", err)
	}
	return certPath, keyPath
}

// writeConfig writes a minimal serverConfig.xml document with the given inner
// XML into a temporary directory and returns its path.
func writeConfig(t *testing.T, inner string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "serverConfig.xml")
	doc := `<?xml version="1.0" encoding="UTF-8"?>` + "\n<serverConfig>" + inner + "</serverConfig>\n"
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("WriteFile config: %v", err)
	}
	return path
}

// TestLoadServerConfig_TLSCertKeyStore confirms that a document carrying a
// <tlsCertKeyStore/> element is parsed and that its key pair is loaded into
// SignerTLSCertKey, ready to sign exam report attachments.
func TestLoadServerConfig_TLSCertKeyStore(t *testing.T) {
	certPath, keyPath := writeTestKeyPair(t, t.TempDir())
	cfgPath := writeConfig(t, `<tlsCertKeyStore certPath="`+certPath+`" keyPath="`+keyPath+`"/>`)

	cfg, err := LoadServerConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadServerConfig: %v", err)
	}
	if cfg.TLSCertKeyStore == nil {
		t.Fatal("TLSCertKeyStore is nil, want the parsed element")
	}
	if cfg.TLSCertKeyStore.CertPath != certPath || cfg.TLSCertKeyStore.KeyPath != keyPath {
		t.Errorf("paths = %q/%q, want %q/%q",
			cfg.TLSCertKeyStore.CertPath, cfg.TLSCertKeyStore.KeyPath, certPath, keyPath)
	}
	if cfg.SignerTLSCertKey == nil {
		t.Fatal("SignerTLSCertKey is nil, want the loaded key pair")
	}
	key, certDER, err := cfg.SignerTLSCertKey.GetKeyPair()
	if err != nil {
		t.Fatalf("GetKeyPair: %v", err)
	}
	if key == nil {
		t.Error("GetKeyPair returned a nil private key")
	}
	if len(certDER) == 0 {
		t.Error("GetKeyPair returned an empty certificate")
	}
}

// TestLoadServerConfig_NoTLSCertKeyStore confirms that a document without a
// <tlsCertKeyStore/> element leaves both fields nil, so attachments stay
// unsigned.
func TestLoadServerConfig_NoTLSCertKeyStore(t *testing.T) {
	cfg, err := LoadServerConfig(writeConfig(t, ""))
	if err != nil {
		t.Fatalf("LoadServerConfig: %v", err)
	}
	if cfg.TLSCertKeyStore != nil {
		t.Errorf("TLSCertKeyStore = %+v, want nil", cfg.TLSCertKeyStore)
	}
	if cfg.SignerTLSCertKey != nil {
		t.Errorf("SignerTLSCertKey = %+v, want nil", cfg.SignerTLSCertKey)
	}
}

// TestLoadServerConfig_TLSCertKeyStoreMissingFiles confirms that a
// <tlsCertKeyStore/> element pointing at unreadable files fails the whole
// configuration load rather than surfacing at the first signed attachment.
func TestLoadServerConfig_TLSCertKeyStoreMissingFiles(t *testing.T) {
	dir := t.TempDir()
	inner := `<tlsCertKeyStore certPath="` + filepath.Join(dir, "no-cert.pem") +
		`" keyPath="` + filepath.Join(dir, "no-key.pem") + `"/>`
	if _, err := LoadServerConfig(writeConfig(t, inner)); err == nil {
		t.Fatal("LoadServerConfig with missing key pair files: got nil error, want failure")
	}
}
