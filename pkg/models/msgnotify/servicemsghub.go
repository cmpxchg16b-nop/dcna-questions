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
	// as the fallback destination when the real recipient of a message
	// cannot be derived. It may be empty.
	sysadminEmail string
}

var _ MsgNotifySvc = (*ServiceMessageHub)(nil)

// NewServiceMessageHub returns a ServiceMessageHub routing through router.
// sysadminEmail is the email address of the system administrator, used as
// the fallback destination when the real recipient of a message cannot be
// derived; it may be empty, in which case such messages are silently
// dropped.
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
// The hub only processes messages from known sources — sources it has a
// defined handling for. A message from any other source is dropped silently:
// process returns nil without consulting the router.
//
// For a message from the exam report server (the sender carries the
// msg_source=exam_report_server label), the destination handed to Send is not
// the real recipient: the real destination is derived from the exam taker's
// email address label, falling back to the sysadmin email address. When
// neither yields an address, process returns nil, silently proceeding.
//
// A message from the exam session server (msg_source=exam_session_server)
// goes to the sysadmin: when the sysadmin email address resolves, it becomes
// the real destination address; otherwise process returns nil, silently
// proceeding.
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

	switch {
	case isFromExamReportServer(replyTo):
		derived, ok := h.deriveDestination(replyTo)
		if !ok {
			// Neither the exam taker's nor the sysadmin's email address is
			// known: silently proceed.
			return nil
		}
		to = derived
	case isFromExamSessionServer(replyTo):
		if h.sysadminEmail == "" {
			// The sysadmin email address is not configured: silently proceed.
			return nil
		}
		to = AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: h.sysadminEmail}
	default:
		// Unknown source: no handling has been defined for it, so the
		// message is dropped silently.
		return nil
	}

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
	return hasMsgSource(replyTo, WellKnownLabelValueExamReportServer)
}

// isFromExamSessionServer reports whether the sender address is labeled as
// the exam session server.
func isFromExamSessionServer(replyTo AddrId) bool {
	return hasMsgSource(replyTo, WellKnownLabelValueExamSessionServer)
}

// hasMsgSource reports whether the address carries a msg_source label with
// the given value.
func hasMsgSource(addr AddrId, source string) bool {
	for _, v := range addr.Tags.GetByLabelKey(WellKnownLabelKeyMsgSource) {
		if v == source {
			return true
		}
	}
	return false
}

// deriveDestination derives the real destination of a message from the exam
// report server: the exam taker's email address label when it carries one,
// otherwise the sysadmin email address. ok is false when neither is known.
func (h *ServiceMessageHub) deriveDestination(replyTo AddrId) (dest AddrId, ok bool) {
	if emails := replyTo.Tags.GetByLabelKey(WellKnownLabelKeyExamTakerExmail); len(emails) > 0 && emails[0] != "" {
		return AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: emails[0]}, true
	}
	if h.sysadminEmail != "" {
		return AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: h.sysadminEmail}, true
	}
	return AddrId{}, false
}
