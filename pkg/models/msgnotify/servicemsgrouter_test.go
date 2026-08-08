package msgnotify

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestOnMemoryMsgRouter_RoutesByDestinationAddressFamily(t *testing.T) {
	emailSvc := &recordingSvc{}
	consoleSvc := &recordingSvc{}
	r := NewOnMemoryMsgRouter([]MsgRoute{
		{DstAddrFamily: MsgNotifyAddrFamilyEmail, NextHop: emailSvc},
		{DstAddrFamily: MsgNotifyAddrFamilyConsole, NextHop: consoleSvc},
	})

	tests := []struct {
		name string
		to   AddrId
		want MsgNotifySvc
	}{
		{"email destination", hubRecipient, emailSvc},
		{"console destination", AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout}, consoleSvc},
		{"unrouted family", AddrId{AddressFamily: MsgNotifyAddrFamilyService, Address: WellKnownAddrServiceOnMemoryExamTrackingServer}, nil},
		{"zero address", AddrId{}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := r.GetNextHop(hubSender, tc.to); got != tc.want {
				t.Errorf("GetNextHop(to=%v) = %v, want %v", tc.to, got, tc.want)
			}
		})
	}
}

func TestOnMemoryMsgRouter_IgnoresTheSenderAddress(t *testing.T) {
	svc := &recordingSvc{}
	r := NewOnMemoryMsgRouter([]MsgRoute{{DstAddrFamily: MsgNotifyAddrFamilyEmail, NextHop: svc}})

	for _, replyTo := range []AddrId{
		hubSender,
		{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStderr},
		{},
	} {
		if got := r.GetNextHop(replyTo, hubRecipient); got != MsgNotifySvc(svc) {
			t.Errorf("GetNextHop(replyTo=%v) = %v, want the email route's hop", replyTo, got)
		}
	}
}

func TestOnMemoryMsgRouter_FirstMatchingRouteWins(t *testing.T) {
	first := &recordingSvc{}
	second := &recordingSvc{}
	r := NewOnMemoryMsgRouter([]MsgRoute{
		{DstAddrFamily: MsgNotifyAddrFamilyEmail, NextHop: first},
		{DstAddrFamily: MsgNotifyAddrFamilyEmail, NextHop: second},
	})

	if got := r.GetNextHop(hubSender, hubRecipient); got != MsgNotifySvc(first) {
		t.Errorf("GetNextHop = %v, want the first matching route's hop", got)
	}
}

func TestOnMemoryMsgRouter_EmptyRouterHasNoRoute(t *testing.T) {
	r := NewOnMemoryMsgRouter(nil)
	if got := r.GetNextHop(hubSender, hubRecipient); got != nil {
		t.Errorf("GetNextHop = %v, want nil for a router without routes", got)
	}
}

// TestNewOnMemoryMsgRouter_CopiesTheRoutesSlice guards against aliasing:
// mutating the caller's slice after construction must not affect the router.
func TestNewOnMemoryMsgRouter_CopiesTheRoutesSlice(t *testing.T) {
	svc := &recordingSvc{}
	routes := []MsgRoute{{DstAddrFamily: MsgNotifyAddrFamilyEmail, NextHop: svc}}
	r := NewOnMemoryMsgRouter(routes)

	routes[0] = MsgRoute{DstAddrFamily: MsgNotifyAddrFamilyEmail, NextHop: &recordingSvc{}}

	if got := r.GetNextHop(hubSender, hubRecipient); got != MsgNotifySvc(svc) {
		t.Errorf("GetNextHop = %v, want the route's original hop", got)
	}
}

// examCompletionSender returns a service-family sender address labeled as an
// exam-completion notification from the exam report server with explicit
// mailing consent, so the hub re-destines the message to the exam taker's
// email address.
func examCompletionSender() AddrId {
	return AddrId{
		AddressFamily: MsgNotifyAddrFamilyService,
		Address:       "exam-tracker",
		Tags: AssociationsList{
			MakeLabelKey(WellKnownLabelKeyMsgSource, WellKnownLabelValueExamReportServer),
			MakeLabelKey(WellKnownLabelKeyExamEvent, WellKnownLabelValueExamCompleted),
			MakeLabelKey(WellKnownLabelKeyExamTakerExmail, "taker@example.com"),
			MakeLabelKey(WellKnownLabelKeyExamReportMailConsent, "true"),
		},
	}
}

// TestOnMemoryMsgRouter_WithServiceMessageHub wires the router behind a real
// hub: an exam-completion message is re-destined to the exam taker's email
// address and lands on the email route's next hop.
func TestOnMemoryMsgRouter_WithServiceMessageHub(t *testing.T) {
	emailSvc := &recordingSvc{}
	hub := NewServiceMessageHub(NewOnMemoryMsgRouter([]MsgRoute{
		{DstAddrFamily: MsgNotifyAddrFamilyEmail, NextHop: emailSvc},
	}), "admin@example.com")

	if err := hub.Send(context.Background(), examCompletionSender(), AddrId{}, hubTestMsg("m1")); err != nil {
		t.Fatalf("Send = %v, want nil", err)
	}
	if len(emailSvc.sent) != 1 {
		t.Fatalf("email svc got %d messages, want 1", len(emailSvc.sent))
	}
	if got := emailSvc.sent[0].to; got.AddressFamily != MsgNotifyAddrFamilyEmail || got.Address != "taker@example.com" {
		t.Errorf("delivered to %v, want email:taker@example.com", got)
	}
	// The sender is derived too: the next hop receives the sysadmin email
	// address as From, not the service-family exam report server address.
	if got := emailSvc.sent[0].replyTo; got.AddressFamily != MsgNotifyAddrFamilyEmail || got.Address != "admin@example.com" {
		t.Errorf("delivered from %v, want email:admin@example.com", got)
	}
}

// TestOnMemoryMsgRouter_WithServiceMessageHub_NoMatchingRoute verifies the
// documented drop behavior: the router has no route for the derived email
// destination, so the hub drops the message and still reports success.
func TestOnMemoryMsgRouter_WithServiceMessageHub_NoMatchingRoute(t *testing.T) {
	consoleSvc := &recordingSvc{}
	hub := NewServiceMessageHub(NewOnMemoryMsgRouter([]MsgRoute{
		{DstAddrFamily: MsgNotifyAddrFamilyConsole, NextHop: consoleSvc},
	}), "admin@example.com")

	if err := hub.Send(context.Background(), examCompletionSender(), AddrId{}, hubTestMsg("m2")); err != nil {
		t.Fatalf("Send = %v, want nil (a missing route is a drop, not an error)", err)
	}
	if len(consoleSvc.sent) != 0 {
		t.Errorf("console svc got %d messages, want 0 (no route for the email destination)", len(consoleSvc.sent))
	}
}

// TestOnMemoryMsgRouter_WithServiceMessageHub_ConsoleFallback verifies the
// non-consent path end to end: a completion message without mailing consent
// is derived to the console destination and lands on the console route's
// next hop.
func TestOnMemoryMsgRouter_WithServiceMessageHub_ConsoleFallback(t *testing.T) {
	consoleSvc := &recordingSvc{}
	hub := NewServiceMessageHub(NewOnMemoryMsgRouter([]MsgRoute{
		{DstAddrFamily: MsgNotifyAddrFamilyConsole, NextHop: consoleSvc},
	}), "admin@example.com")

	// Same as examCompletionSender but without the mailing-consent label.
	sender := AddrId{
		AddressFamily: MsgNotifyAddrFamilyService,
		Address:       "exam-tracker",
		Tags: AssociationsList{
			MakeLabelKey(WellKnownLabelKeyMsgSource, WellKnownLabelValueExamReportServer),
			MakeLabelKey(WellKnownLabelKeyExamEvent, WellKnownLabelValueExamCompleted),
			MakeLabelKey(WellKnownLabelKeyExamTakerExmail, "taker@example.com"),
		},
	}
	if err := hub.Send(context.Background(), sender, AddrId{}, hubTestMsg("m3")); err != nil {
		t.Fatalf("Send = %v, want nil", err)
	}
	if len(consoleSvc.sent) != 1 {
		t.Fatalf("console svc got %d messages, want 1", len(consoleSvc.sent))
	}
	want := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout}
	if got := consoleSvc.sent[0].to; !got.AddrEqual(want) {
		t.Errorf("delivered to %v, want %v", got, want)
	}
}

func TestCatchAllServiceMsgRouter_BrandNewSinkPerQuery(t *testing.T) {
	r := CatchAllServiceMsgRouter{}

	first := r.GetNextHop(hubSender, hubRecipient)
	second := r.GetNextHop(hubSender, hubRecipient)
	if first == nil || second == nil {
		t.Fatalf("GetNextHop returned nil: first=%v second=%v", first, second)
	}
	if first == second {
		t.Error("GetNextHop returned the same sink instance twice, want a brand new one per query")
	}
	if _, ok := first.(*CatchAllServiceMsgSink); !ok {
		t.Errorf("GetNextHop returned %T, want *CatchAllServiceMsgSink", first)
	}
}

func TestCatchAllServiceMsgSink_AcceptsEverything(t *testing.T) {
	sink := &CatchAllServiceMsgSink{}

	for _, addr := range []AddrId{hubSender, hubRecipient,
		{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout},
		{},
	} {
		if !sink.AreYou(addr) {
			t.Errorf("AreYou(%v) = false, want true", addr)
		}
	}

	want := map[MsgNotifyAddrFamily]bool{
		MsgNotifyAddrFamilyConsole: true,
		MsgNotifyAddrFamilyEmail:   true,
		MsgNotifyAddrFamilyService: true,
	}
	for _, fams := range [][]MsgNotifyAddrFamily{
		sink.GetAcceptedSenderAddressFamilies(),
		sink.GetAcceptedRecipientAddressFamilies(),
	} {
		for _, f := range fams {
			delete(want, f)
		}
	}
	for f := range want {
		t.Errorf("family %q not accepted by the catch-all sink", f)
	}
}

func TestCatchAllServiceMsgSink_LogsInsteadOfDelivering(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	sink := &CatchAllServiceMsgSink{}
	msg := hubTestMsg("sink-1")
	if err := sink.Send(context.Background(), hubSender, hubRecipient, msg); err != nil {
		t.Fatalf("Send = %v, want nil", err)
	}
	out := buf.String()
	for _, want := range []string{"sink-1", msg.Text} {
		if !strings.Contains(out, want) {
			t.Errorf("log output = %q, want it to mention %q", out, want)
		}
	}
}
