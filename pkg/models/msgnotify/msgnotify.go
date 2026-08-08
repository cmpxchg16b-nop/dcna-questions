package msgnotify

import (
	"context"
	"strings"
)

// MsgNotifyAddrFamily identifies the addressing scheme an AddrId belongs to.
type MsgNotifyAddrFamily string

const (
	MsgNotifyAddrFamilyConsole MsgNotifyAddrFamily = "console"
	MsgNotifyAddrFamilyEmail   MsgNotifyAddrFamily = "email"
	MsgNotifyAddrFamilyService MsgNotifyAddrFamily = "service"
)

const (
	// WellKnownAddrConsoleStdout is the well-known console address for standard output.
	WellKnownAddrConsoleStdout = "/dev/stdout"

	// WellKnownAddrConsoleStderr is the well-known console address for standard error.
	WellKnownAddrConsoleStderr = "/dev/stderr"
)

// Well-known service addresses, in the service address family.
const (
	// WellKnownAddrServiceOnMemoryExamTrackingServer is the well-known
	// service address of the on-memory exam tracking server.
	WellKnownAddrServiceOnMemoryExamTrackingServer = "24d2b7e6-0fc1-497b-91b7-3d1e7d4af44c"

	// WellKnownAddrServiceOnMemoryExamSessionServer is the well-known
	// service address of the on-memory exam session server.
	WellKnownAddrServiceOnMemoryExamSessionServer = "14383402-9150-4d77-94a7-06d4c56e9814"
)

// AssociationsList is a list of associations (tags) attached to an address.
// The elements of the slice should never be mutated.
type AssociationsList []string

// Get returns the element at index i; hit reports whether i is within bounds.
func (l AssociationsList) Get(i int) (val string, hit bool) {
	if i < 0 || i >= len(l) {
		return "", false
	}
	return l[i], true
}

// GetAll returns all elements of the list. The returned slice is never
// expected to be mutated.
func (l AssociationsList) GetAll() []string {
	return l
}

// MakeLabelKey encodes a label key/value pair into a tag string, so that
// callers never need to hardcode the tag encoding. It is the inverse of
// GetByLabelKey: l.GetByLabelKey(key) on a list containing
// MakeLabelKey(key, value) yields value.
func MakeLabelKey(labelKey, labelValue string) string {
	return labelKey + "=" + labelValue
}

// GetByLabelKey returns the values of all tags of the form "key=value" that
// match the given key: a tag matches when it has the prefix key+"=", and its
// value is the remainder of the tag, returned as-is without trimming. When no
// tag matches, the result is nil.
func (l AssociationsList) GetByLabelKey(key string) []string {
	prefix := key + "="
	var out []string
	for _, tag := range l {
		if strings.HasPrefix(tag, prefix) {
			out = append(out, tag[len(prefix):])
		}
	}
	return out
}

// Well-known label keys, for use with MakeLabelKey and GetByLabelKey.
const (
	// WellKnownLabelKeyMsgSource labels the source of a message.
	WellKnownLabelKeyMsgSource = "msg_source"

	// WellKnownLabelKeyExamTakerSubjectId labels the subject id of the exam taker.
	WellKnownLabelKeyExamTakerSubjectId = "examtaker_subject_id"

	// WellKnownLabelKeyExamTakerExmail labels the email address of the exam taker.
	WellKnownLabelKeyExamTakerExmail = "examtaker_email"

	// WellKnownLabelKeyExamTakerUsername labels the username of the exam taker.
	WellKnownLabelKeyExamTakerUsername = "examtaker_username"

	// WellKnownLabelKeyExamTakerFirstName labels the first name of the exam taker.
	WellKnownLabelKeyExamTakerFirstName = "examtaker_first_name"

	// WellKnownLabelKeyExamTakerLastName labels the last name of the exam taker.
	WellKnownLabelKeyExamTakerLastName = "examtaker_lastname"

	// WellKnownLabelKeyExamOverallResult labels the overall result of the
	// exam assessment.
	WellKnownLabelKeyExamOverallResult = "exam_overall_result"

	// WellKnownLabelKeyExamReportMailConsent labels the exam taker's consent
	// to the exam report being emailed to the exam taker's email address. Its
	// value is a strconv.FormatBool boolean.
	WellKnownLabelKeyExamReportMailConsent = "examreport_mail_consent"

	// WellKnownLabelKeyExamEvent labels the exam lifecycle event a
	// notification reports; its value is one of the WellKnownLabelValueExam*
	// constants.
	WellKnownLabelKeyExamEvent = "exam_event"
)

// Well-known label values, for use with MakeLabelKey.
const (
	// WellKnownLabelValueExamReportServer labels a message as originating
	// from the exam report server.
	WellKnownLabelValueExamReportServer = "exam_report_server"

	// WellKnownLabelValueExamSessionServer labels a message as originating
	// from the exam session server.
	WellKnownLabelValueExamSessionServer = "exam_session_server"

	// WellKnownLabelValueExamCompleted labels a notification as reporting a
	// completed exam session: an exam report was recorded.
	WellKnownLabelValueExamCompleted = "exam_completed"

	// WellKnownLabelValueExamReportDeleted labels a notification as reporting
	// the deletion of an exam report.
	WellKnownLabelValueExamReportDeleted = "exam_report_deleted"
)

// AddrId identifies a messaging endpoint (sender or recipient).
type AddrId struct {
	AddressFamily MsgNotifyAddrFamily
	Address       string

	// Tags is the list of associations attached to this address. It is
	// readonly: it should never be mutated.
	Tags AssociationsList
}

// AddrEqual reports whether a and b identify the same endpoint: two AddrIds
// are AddrEqual if and only if they have identical AddressFamily and
// identical Address. Tags are not part of the comparison.
func (a AddrId) AddrEqual(b AddrId) bool {
	return a.AddressFamily == b.AddressFamily && a.Address == b.Address
}

// MessageLevel indicates the importance of a message.
type MessageLevel int

const (
	MessageLevelImportant MessageLevel = iota
	MessageLevelCommon
)

// BlobAttachment is a binary attachment carried by a Msg.
type BlobAttachment struct {
	Id       string
	Content  []byte
	MIMEType string
	Size     int
	Filename string
}

// Msg is a notification message to be delivered.
type Msg struct {
	// Id is the globally unique identifier of the message.
	Id string

	// Created is the millisecond-resolution unix timestamp when the message
	// was created.
	Created int64

	Title string
	Level MessageLevel

	// Text is the plaintext content of the message.
	Text string

	// HTML is the optional rich-text (HTML) content of the message. When set,
	// services capable of rendering HTML (such as the email service) deliver
	// it alongside Text, which remains the plain-text fallback for clients
	// that do not render HTML.
	HTML string

	// Attachments are the binary attachments carried by the message. It is
	// up to each MsgNotifySvc implementation whether attachments are
	// delivered.
	Attachments []BlobAttachment
}

// NodeLike is the identity aspect of a messaging node: a node can be asked
// whether it is (owns) a given address.
type NodeLike interface {
	// AreYou reports whether the node identifies as addrId.
	AreYou(addrId AddrId) bool
}

// MsgNotifySvc is the messaging notification service interface.
type MsgNotifySvc interface {
	// A MsgNotifySvc is also NodeLike-capable.
	NodeLike

	// GetAcceptedSenderAddressFamilies returns the address families the
	// service accepts for the sender (replyTo) address. Callers can use it to
	// negotiate with the service before attempting Send.
	GetAcceptedSenderAddressFamilies() []MsgNotifyAddrFamily

	// GetAcceptedRecipientAddressFamilies returns the address families the
	// service accepts for the recipient (to) address. Callers can use it to
	// negotiate with the service before attempting Send.
	GetAcceptedRecipientAddressFamilies() []MsgNotifyAddrFamily

	// Send delivers msg to the recipient 'to', with 'replyTo' as the
	// address replies should be directed to.
	Send(ctx context.Context, replyTo AddrId, to AddrId, msg Msg) error
}

var _ NodeLike = MsgNotifySvc(nil)
