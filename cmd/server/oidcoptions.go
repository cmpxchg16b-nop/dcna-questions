package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"time"
)

// oidcLoginOptionsXML mirrors the structure of oidcLoginOptions.xml (validated
// against oidcLoginOptions.xsd in the project root). Each <oidcLoginOption/>
// maps onto the configuration fields of GenericOIDCLoginHandler.
type oidcLoginOptionsXML struct {
	XMLName xml.Name             `xml:"oidcLoginOptions"`
	Options []oidcLoginOptionXML `xml:"oidcLoginOption"`
}

type oidcLoginOptionXML struct {
	ProviderName            string `xml:"providerName,attr"`
	IssuerURL               string `xml:"issuerURL,attr"`
	ClientId                string `xml:"clientId,attr"`
	ClientSecret            string `xml:"clientSecret,attr"`
	RedirectURL             string `xml:"redirectURL,attr"`
	Scope                   string `xml:"scope,attr"`
	SessionLifespan         string `xml:"sessionLifespan,attr"`
	LoginSuccessRedirectURL string `xml:"loginSuccessRedirectURL,attr"`
}

// loadOIDCLoginOptions parses the OIDC login options XML document.
func loadOIDCLoginOptions(path string) (*oidcLoginOptionsXML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read OIDC login options file %s: %w", path, err)
	}
	var opts oidcLoginOptionsXML
	if err := xml.Unmarshal(data, &opts); err != nil {
		return nil, fmt.Errorf("failed to parse OIDC login options file %s: %w", path, err)
	}
	return &opts, nil
}

// parseSessionLifespan parses a Go time.Duration string, falling back to the
// given default when the input is empty.
func parseSessionLifespan(s string, fallback time.Duration) (time.Duration, error) {
	if s == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	return d, nil
}
