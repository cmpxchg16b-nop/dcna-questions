package msgnotify

import (
	"context"
	"log/slog"
	"slices"
	"sync/atomic"
)

// OnMemoryMsgRouter is a ServiceMessageRouter that chooses the next hop of a
// message solely from the address family of the destination address: the
// first route whose DstAddrFamily matches wins.
//
// The routes are fixed at construction and never mutated afterwards, so
// concurrent GetNextHop calls are safe without locking.
type OnMemoryMsgRouter struct {
	routes []MsgRoute
}

var _ ServiceMessageRouter = (*OnMemoryMsgRouter)(nil)

// NewOnMemoryMsgRouter returns an OnMemoryMsgRouter consulting routes in
// order: the first route whose DstAddrFamily matches the destination address
// family of a message becomes its next hop. The slice is cloned, so later
// mutation of the caller's slice does not affect the router.
func NewOnMemoryMsgRouter(routes []MsgRoute) *OnMemoryMsgRouter {
	return &OnMemoryMsgRouter{routes: slices.Clone(routes)}
}

// GetNextHop returns the NextHop of the first route matching the destination
// address family, or nil when no route matches.
func (r *OnMemoryMsgRouter) GetNextHop(replyToAddr, toAddr AddrId) MsgNotifySvc {
	for _, route := range r.routes {
		if route.DstAddrFamily == toAddr.AddressFamily {
			return route.NextHop
		}
	}
	return nil
}

// CatchAllServiceMsgRouter is a ServiceMessageRouter that answers every query
// with a brand new CatchAllServiceMsgSink.
type CatchAllServiceMsgRouter struct{}

var _ ServiceMessageRouter = CatchAllServiceMsgRouter{}

// catchAllSinkSeq numbers the CatchAllServiceMsgSink instances handed out by
// CatchAllServiceMsgRouter.
var catchAllSinkSeq atomic.Uint64

// GetNextHop returns a brand new CatchAllServiceMsgSink for every query.
func (CatchAllServiceMsgRouter) GetNextHop(replyToAddr, toAddr AddrId) MsgNotifySvc {
	return &CatchAllServiceMsgSink{id: catchAllSinkSeq.Add(1)}
}

// CatchAllServiceMsgSink is a MsgNotifySvc dead end: it accepts addresses of
// every known family and claims every AreYou probe, but it never delivers a
// message to the real recipient — it simply logs the message.
type CatchAllServiceMsgSink struct {
	// id uniquely identifies the sink instance. Besides appearing in the
	// logs, it gives the struct a non-zero size: without it the compiler
	// could alias every instance to the same address, defeating the
	// brand-new-instance-per-query guarantee of CatchAllServiceMsgRouter.
	id uint64
}

var _ MsgNotifySvc = (*CatchAllServiceMsgSink)(nil)

// AreYou always answers yes.
func (s *CatchAllServiceMsgSink) AreYou(addrId AddrId) bool {
	return true
}

// GetAcceptedSenderAddressFamilies returns all known address families.
func (s *CatchAllServiceMsgSink) GetAcceptedSenderAddressFamilies() []MsgNotifyAddrFamily {
	return []MsgNotifyAddrFamily{
		MsgNotifyAddrFamilyConsole,
		MsgNotifyAddrFamilyEmail,
		MsgNotifyAddrFamilyService,
	}
}

// GetAcceptedRecipientAddressFamilies returns all known address families.
func (s *CatchAllServiceMsgSink) GetAcceptedRecipientAddressFamilies() []MsgNotifyAddrFamily {
	return []MsgNotifyAddrFamily{
		MsgNotifyAddrFamilyConsole,
		MsgNotifyAddrFamilyEmail,
		MsgNotifyAddrFamilyService,
	}
}

// Send logs the message instead of delivering it to the real recipient.
func (s *CatchAllServiceMsgSink) Send(ctx context.Context, replyTo AddrId, to AddrId, msg Msg) error {
	slog.InfoContext(ctx, "msgnotify: catch-all sink swallowed message",
		"sinkId", s.id, "messageId", msg.Id, "title", msg.Title, "level", msg.Level,
		"from", replyTo, "to", to, "text", msg.Text)
	return nil
}
