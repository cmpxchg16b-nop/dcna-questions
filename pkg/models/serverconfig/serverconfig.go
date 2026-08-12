// Package serverconfig parses serverConfig.xml, the global server
// configuration document validated against serverConfig.xsd in the project
// root.
package serverconfig

import (
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"os"
	"time"

	dsig "github.com/russellhaering/goxmldsig"
)

// ServerConfigXML mirrors the structure of serverConfig.xml (validated
// against serverConfig.xsd in the project root), the global server
// configuration document.
type ServerConfigXML struct {
	XMLName          xml.Name            `xml:"serverConfig"`
	OIDCLoginOptions OIDCLoginOptionsXML `xml:"oidcLoginOptions"`
	// SMTPServer is nil when the document has no <smtpServer/> section.
	SMTPServer *SMTPServerXML `xml:"smtpServer"`
	// TLSCertKeyStore is nil when the document has no <tlsCertKeyStore/>
	// element.
	TLSCertKeyStore *TLSCertKeyStoreXML `xml:"tlsCertKeyStore"`
	LoginOptions    LoginOptionsXML     `xml:"loginOptions"`
	// AllowedOrigins holds every <allowedOrigin/> entry of the document: the
	// request origins trusted by the OAuth2/OIDC login handlers when their
	// configured redirect URL is relative (see pkg/api/common
	// ResolveRedirectURL).
	AllowedOrigins []string `xml:"allowedOrigin"`

	// SignerTLSCertKey is the XMLDSIG signing key pair LoadServerConfig loaded
	// from the <tlsCertKeyStore/> element's certPath/keyPath; it is nil when
	// the document has no such element. *dsig.TLSCertKeyStore implements
	// dsig.X509KeyStore, so it can be handed directly to
	// examreport.NewOnMemoryExamTrackingServer.
	SignerTLSCertKey *dsig.TLSCertKeyStore `xml:"-"`
}

// OIDCLoginOptionsXML mirrors the <oidcLoginOptions/> section of
// serverConfig.xml. Each <oidcLoginOption/> maps onto the configuration
// fields of GenericOIDCLoginHandler.
type OIDCLoginOptionsXML struct {
	Options []OIDCLoginOptionXML `xml:"oidcLoginOption"`
}

// OIDCLoginOptionXML mirrors a single <oidcLoginOption/> entry of the
// <oidcLoginOptions/> section of serverConfig.xml.
type OIDCLoginOptionXML struct {
	ProviderName            string `xml:"providerName,attr"`
	IssuerURL               string `xml:"issuerURL,attr"`
	ClientId                string `xml:"clientId,attr"`
	ClientSecret            string `xml:"clientSecret,attr"`
	RedirectURL             string `xml:"redirectURL,attr"`
	Scope                   string `xml:"scope,attr"`
	SessionLifespan         string `xml:"sessionLifespan,attr"`
	LoginSuccessRedirectURL string `xml:"loginSuccessRedirectURL,attr"`
}

// SMTPServerXML mirrors the <smtpServer/> section of serverConfig.xml: the
// outbound SMTP service used to deliver notification messages. It maps onto
// EmailBasedMsgSvcInitOption in pkg/models/msgnotify.
//
// StartTLS and TLS select the transport security and are mutually exclusive
// (enforced by the wiring, not the schema); when both are false the
// connection is plaintext. When Username is empty the server is used without
// authentication.
type SMTPServerXML struct {
	Host     string `xml:"host,attr"`
	Port     int    `xml:"port,attr"`
	Username string `xml:"username,attr"`
	Password string `xml:"password,attr"`
	StartTLS bool   `xml:"startTLS,attr"`
	TLS      bool   `xml:"tls,attr"`
}

// TLSCertKeyStoreXML mirrors the <tlsCertKeyStore/> element of
// serverConfig.xml: the filesystem paths of the X.509 certificate and private
// key the exam tracking server signs XMLDSIG-enveloped exam report
// attachments with. Both paths point to PEM-encoded files loadable with
// tls.LoadX509KeyPair; only RSA keys are usable for XMLDSIG signing.
type TLSCertKeyStoreXML struct {
	CertPath string `xml:"certPath,attr"`
	KeyPath  string `xml:"keyPath,attr"`
}

// LoginOptionsXML mirrors the <loginOptions/> section of serverConfig.xml:
// the login page's configurable IdP list, served to the frontend as JSON by
// the login options API handler (pkg/api/loginoptions).
type LoginOptionsXML struct {
	Options []LoginOptionXML `xml:"loginOption"`
}

// LoginOptionXML mirrors a single <loginOption/> entry of the
// <loginOptions/> section of serverConfig.xml.
type LoginOptionXML struct {
	Kind        string `xml:"kind,attr"`
	Name        string `xml:"name,attr"`
	DisplayName string `xml:"displayName,attr"`
	Label       string `xml:"label,attr"`
	LoginURL    string `xml:"loginURL,attr"`
	// AllowedOrigins is the raw comma-separated allowedOrigins attribute;
	// an empty string means no origin restriction. Split it with
	// loginoptions.ParseAllowedOrigins.
	AllowedOrigins string `xml:"allowedOrigins,attr"`
}

// LoadServerConfig parses the global server configuration XML document.
func LoadServerConfig(path string) (*ServerConfigXML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read server config file %s: %w", path, err)
	}
	var cfg ServerConfigXML
	if err := xml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse server config file %s: %w", path, err)
	}
	// The optional <tlsCertKeyStore/> element names the key pair the exam
	// tracking server signs report attachments with. Load it now so an
	// unreadable or mismatched pair fails at startup instead of at the first
	// signed attachment.
	if cfg.TLSCertKeyStore != nil {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertKeyStore.CertPath, cfg.TLSCertKeyStore.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load <tlsCertKeyStore/> key pair of server config file %s: %w", path, err)
		}
		keyStore := dsig.TLSCertKeyStore(cert)
		cfg.SignerTLSCertKey = &keyStore
		// goxmldsig signs with RSA keys only: surface an unusable key now
		// rather than when the first attachment is signed.
		if _, _, err := cfg.SignerTLSCertKey.GetKeyPair(); err != nil {
			return nil, fmt.Errorf("unusable <tlsCertKeyStore/> key pair in server config file %s: %w", path, err)
		}
	}
	return &cfg, nil
}

// ParseSessionLifespan parses a Go time.Duration string, falling back to the
// given default when the input is empty.
func ParseSessionLifespan(s string, fallback time.Duration) (time.Duration, error) {
	if s == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	return d, nil
}
