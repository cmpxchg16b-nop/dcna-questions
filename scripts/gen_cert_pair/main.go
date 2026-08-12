// Command gen_cert_pair creates a self-signed X.509 certificate and RSA
// private key pair in PEM form, suitable for the <tlsCertKeyStore/> element
// of serverConfig.xml: the exam tracking server uses the pair to envelop
// XMLDSIG signatures into the exam report attachments it emails, and
// /api/certs/verify trusts the certificate when verifying those signatures.
//
// goxmldsig signs with RSA keys only, so the key type is not configurable.
//
// Usage:
//
//	go run ./scripts/gen_cert_pair -out ./signing -cn "Example Exam Center"
//
// writes ./signing/cert.pem and ./signing/key.pem (mode 0600) and prints the
// <tlsCertKeyStore/> snippet to paste into serverConfig.xml.
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	outDir := flag.String("out", ".", "output directory for cert.pem and key.pem")
	cn := flag.String("cn", "dcna exam report signer", "common name (CN) of the self-signed certificate")
	org := flag.String("org", "", "organization (O) of the certificate (optional)")
	days := flag.Int("days", 3650, "validity of the certificate in days")
	bits := flag.Int("bits", 3072, "RSA key size in bits")
	flag.Parse()

	key, err := rsa.GenerateKey(rand.Reader, *bits)
	if err != nil {
		log.Fatalf("generating RSA key: %v", err)
	}

	// A random 128-bit serial number, as recommended by CABF for certs in
	// general; harmless for a self-signed signer cert.
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		log.Fatalf("generating serial number: %v", err)
	}
	subject := pkix.Name{CommonName: *cn}
	if *org != "" {
		subject.Organization = []string{*org}
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               subject,
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.AddDate(0, 0, *days),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		log.Fatalf("creating certificate: %v", err)
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("creating output directory %s: %v", *outDir, err)
	}
	certPath := filepath.Join(*outDir, "cert.pem")
	keyPath := filepath.Join(*outDir, "key.pem")
	certOut := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyOut := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(certPath, certOut, 0o644); err != nil {
		log.Fatalf("writing %s: %v", certPath, err)
	}
	// The private key signs every exam report: keep it owner-only.
	if err := os.WriteFile(keyPath, keyOut, 0o600); err != nil {
		log.Fatalf("writing %s: %v", keyPath, err)
	}

	fmt.Printf("wrote %s and %s\n", certPath, keyPath)
	fmt.Printf("subject:            %s\n", subject.String())
	fmt.Printf("validity:           %s .. %s\n",
		tmpl.NotBefore.Format(time.RFC3339), tmpl.NotAfter.Format(time.RFC3339))
	fmt.Printf("SHA-256 fingerprint: %s\n", colonHex(der))
	fmt.Println()
	fmt.Println("Add to serverConfig.xml:")
	absCert, err := filepath.Abs(certPath)
	if err != nil {
		absCert = certPath
	}
	absKey, err := filepath.Abs(keyPath)
	if err != nil {
		absKey = keyPath
	}
	fmt.Printf("  <tlsCertKeyStore certPath=%q keyPath=%q />\n", absCert, absKey)
}

// colonHex renders the certificate DER's SHA-256 fingerprint as uppercase hex
// byte pairs joined by colons, matching the fingerprint /api/certs/verify
// reports and the verify dialog shows.
func colonHex(der []byte) string {
	sum := sha256.Sum256(der)
	pairs := make([]string, 0, len(sum))
	for _, b := range sum {
		pairs = append(pairs, fmt.Sprintf("%02X", b))
	}
	return strings.Join(pairs, ":")
}
