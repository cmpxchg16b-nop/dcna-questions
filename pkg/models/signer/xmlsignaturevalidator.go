package signer

import "github.com/beevik/etree"

// XMLETreeSignatureValidator verifies the enveloped signature of an XML
// element tree.
//
// *dsig.ValidationContext from github.com/russellhaering/goxmldsig satisfies
// this interface: its Validate method has exactly this signature, so a
// validation context built from an X509CertificateStore (e.g. via
// dsig.NewDefaultValidationContext) can be used directly.
type XMLETreeSignatureValidator interface {
	// Validate verifies that el carries a valid enveloped XMLDSIG signature
	// and returns the validated element (the copy of el with the Signature
	// removed). It returns an error when the signature is missing, malformed,
	// or does not verify against the validator's trusted certificates.
	Validate(el *etree.Element) (*etree.Element, error)
}
