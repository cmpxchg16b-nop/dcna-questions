package msgnotify

import (
	"context"
	"fmt"
	"log/slog"
)

// ServiceMessageRouter selects the next hop for a message passing through a
// ServiceMessageHub.
type ServiceMessageRouter interface {
	// GetNextHop returns the MsgNotifySvc that should deliver a message sent
	// from replyToAddr to toAddr.
	//
	// GetNextHop must never fail: it has no error return, and the hub never
	// recovers a goroutine that panics inside GetNextHop — a panicking
	// router takes the caller's goroutine down with it. Returning nil is the
	// only way to signal "no route"; the hub then drops the message and
	// simply logs the event.
	GetNextHop(replyToAddr, toAddr AddrId) MsgNotifySvc
}

// MsgRoute binds a destination address family to the MsgNotifySvc that
// delivers messages for it. A nil NextHop is a black hole: it is returned
// as-is, and the hub treats it exactly like an absent route, dropping the
// message.
type MsgRoute struct {
	// DstAddrFamily is the destination address family this route matches.
	DstAddrFamily MsgNotifyAddrFamily

	// NextHop is the MsgNotifySvc that delivers messages whose destination
	// address family matches DstAddrFamily.
	NextHop MsgNotifySvc
}

// ServiceMessageHub is a MsgNotifySvc that forwards messages to other
// MsgNotifySvc implementations selected by a ServiceMessageRouter. Being a
// MsgNotifySvc itself, a ServiceMessageHub is accepted anywhere a MsgNotifySvc
// is expected.
//
// The hub accepts recipients of every known address family, but only senders
// in the service address family: only a service is allowed to send messages
// to a ServiceMessageHub.
type ServiceMessageHub struct {
	router ServiceMessageRouter
	// sysadminEmail is the email address of the system administrator, used
	// as the sender (From) address of email-destined messages. It may be
	// empty, in which case the original service sender address passes
	// through to the next hop unchanged.
	sysadminEmail string
}

var _ MsgNotifySvc = (*ServiceMessageHub)(nil)

// NewServiceMessageHub returns a ServiceMessageHub routing through router.
// sysadminEmail is the email address of the system administrator, used as
// the sender (From) address of email-destined messages; it may be empty, in
// which case the original service sender address passes through unchanged.
func NewServiceMessageHub(router ServiceMessageRouter, sysadminEmail string) *ServiceMessageHub {
	return &ServiceMessageHub{router: router, sysadminEmail: sysadminEmail}
}

// AreYou always answers yes: the hub claims any address it is asked about.
func (h *ServiceMessageHub) AreYou(addrId AddrId) bool {
	return true
}

// GetAcceptedSenderAddressFamilies returns only the service family: only a
// service is allowed to send messages to a ServiceMessageHub.
func (h *ServiceMessageHub) GetAcceptedSenderAddressFamilies() []MsgNotifyAddrFamily {
	return []MsgNotifyAddrFamily{MsgNotifyAddrFamilyService}
}

// GetAcceptedRecipientAddressFamilies returns all known address families.
func (h *ServiceMessageHub) GetAcceptedRecipientAddressFamilies() []MsgNotifyAddrFamily {
	return []MsgNotifyAddrFamily{
		MsgNotifyAddrFamilyConsole,
		MsgNotifyAddrFamilyEmail,
		MsgNotifyAddrFamilyService,
	}
}

// Send validates the sender address family and hands the message to process,
// which selects the real destination and delivers it through the next hop.
func (h *ServiceMessageHub) Send(ctx context.Context, replyTo AddrId, to AddrId, msg Msg) error {
	if replyTo.AddressFamily != MsgNotifyAddrFamilyService {
		return fmt.Errorf("msgnotify: hub: unsupported sender address family %q", replyTo.AddressFamily)
	}
	return h.process(ctx, replyTo, to, msg)
}

// process selects the real destination of the message and delivers it through
// the next hop chosen by the router.
//
// The hub only processes exam-completion notifications: messages from the
// exam report server (the sender carries the msg_source=exam_report_server
// label) that also carry the exam_event=exam_completed label. Every other
// message — session-started events, report deletions, unknown sources — is
// dropped silently: process returns nil without consulting the router. Such
// messages still reach the hub, so a future routing rule can steer them
// somewhere other than a mailbox without touching the emitters.
//
// For an exam-completion notification, the destination handed to Send is not
// the real recipient: the real destination is the exam taker's email address
// label, but only when the message carries an explicit
// examreport_mail_consent=true label — anything else ("false", empty,
// missing, or non-boolean) is not consent. Without consent the destination
// is the well-known stdout console address instead of a mailbox; whether the
// message goes anywhere then depends on the router having a next hop for the
// console address family.
//
// The sender address handed to the next hop is derived too: it becomes the
// sysadmin email address, so an email next hop receives an email-family From
// address. When the sysadmin email address is not configured, the original
// sender address is passed through unchanged.
//
// When there is no router, or no next hop for the destination, the message is
// dropped and the event is logged; process still reports success in that
// case.
func (h *ServiceMessageHub) process(ctx context.Context, replyTo AddrId, to AddrId, msg Msg) error {
	if h.router == nil {
		slog.WarnContext(ctx, "msgnotify: hub has no router, dropping message",
			"messageId", msg.Id, "from", replyTo, "to", to)
		return nil
	}

	if !isFromExamReportServer(replyTo) || !isExamCompletionEvent(replyTo) {
		// Not an exam-completion notification: no routing is defined for it,
		// so the message is dropped silently.
		return nil
	}

	to = h.deriveDestination(replyTo)
	replyTo = h.deriveSender(replyTo)

	nextHop := h.router.GetNextHop(replyTo, to)
	if nextHop == nil {
		slog.WarnContext(ctx, "msgnotify: no next hop for message, dropping",
			"messageId", msg.Id, "from", replyTo, "to", to)
		return nil
	}
	return nextHop.Send(ctx, replyTo, to, msg)
}

// isFromExamReportServer reports whether the sender address is labeled as the
// exam report server.
func isFromExamReportServer(replyTo AddrId) bool {
	return hasLabelValue(replyTo, WellKnownLabelKeyMsgSource, WellKnownLabelValueExamReportServer)
}

// isExamCompletionEvent reports whether the sender address carries an
// exam_event label marking the message as an exam-completion notification.
func isExamCompletionEvent(replyTo AddrId) bool {
	return hasLabelValue(replyTo, WellKnownLabelKeyExamEvent, WellKnownLabelValueExamCompleted)
}

// hasLabelValue reports whether the address carries a label with the given
// key and value.
func hasLabelValue(addr AddrId, key, value string) bool {
	for _, v := range addr.Tags.GetByLabelKey(key) {
		if v == value {
			return true
		}
	}
	return false
}

// deriveDestination derives the real destination of a message from the exam
// report server: the exam taker's email address label, but only when the
// message carries explicit mailing consent; otherwise the well-known stdout
// console address. The console fallback always resolves, so a destination is
// always derived; whether the message goes anywhere then depends on the
// router having a next hop for the console address family.
func (h *ServiceMessageHub) deriveDestination(replyTo AddrId) AddrId {
	if mailingConsentGiven(replyTo) {
		if emails := replyTo.Tags.GetByLabelKey(WellKnownLabelKeyExamTakerExmail); len(emails) > 0 && emails[0] != "" {
			return AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: emails[0]}
		}
	}
	return AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout}
}

// mailingConsentGiven reports whether the sender address carries an explicit
// examreport_mail_consent=true label. Only the exact string "true" (what
// strconv.FormatBool produces) counts as consent: a missing, empty, or any
// other value is not consent.
func mailingConsentGiven(replyTo AddrId) bool {
	return hasLabelValue(replyTo, WellKnownLabelKeyExamReportMailConsent, "true")
}

// deriveSender derives the sender address handed to the next hop for a
// service-originated message: the sysadmin email address, so that an email
// next hop receives an email-family From address. When the sysadmin email
// address is not configured, the original sender address passes through
// unchanged (services that tolerate any sender family, such as the console
// sink, still accept the message).
func (h *ServiceMessageHub) deriveSender(replyTo AddrId) AddrId {
	if h.sysadminEmail == "" {
		return replyTo
	}
	return AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: h.sysadminEmail}
}
