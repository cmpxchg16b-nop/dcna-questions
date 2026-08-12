// Package signer defines the narrow signing abstractions used across the
// server, so components that need XML documents signed depend on a minimal
// interface rather than on a concrete signing implementation or on key
// material handling.
package signer

import "github.com/beevik/etree"

// XMLETreeSigner envelops a signature into an XML element tree.
//
// *dsig.SigningContext from github.com/russellhaering/goxmldsig satisfies
// this interface: its SignEnveloped method has exactly this signature, so a
// signing context built from an X509KeyStore (e.g. via
// dsig.NewDefaultSigningContext) can be used directly.
type XMLETreeSigner interface {
	// SignEnveloped envelops an XMLDSIG signature into el and returns the
	// signed element. The input element is not mutated: the returned element
	// is a copy of el carrying the appended Signature child.
	SignEnveloped(el *etree.Element) (signed *etree.Element, err error)
}
