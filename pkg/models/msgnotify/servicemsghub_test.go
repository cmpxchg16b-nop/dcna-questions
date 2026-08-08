package msgnotify

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// hubSentMessage records one Send invocation of a recordingSvc.
type hubSentMessage struct {
	replyTo AddrId
	to      AddrId
	msg     Msg
}

// recordingSvc is a MsgNotifySvc that records the messages it is asked to
// send and returns a configurable error. areYou is the answer it gives to
// AreYou probes.
type recordingSvc struct {
	sent   []hubSentMessage
	err    error
	areYou bool
}

func (r *recordingSvc) AreYou(addrId AddrId) bool                                  { return r.areYou }
func (r *recordingSvc) GetAcceptedSenderAddressFamilies() []MsgNotifyAddrFamily    { return nil }
func (r *recordingSvc) GetAcceptedRecipientAddressFamilies() []MsgNotifyAddrFamily { return nil }

func (r *recordingSvc) Send(ctx context.Context, replyTo AddrId, to AddrId, msg Msg) error {
	r.sent = append(r.sent, hubSentMessage{replyTo: replyTo, to: to, msg: msg})
	return r.err
}

// staticRouter is a ServiceMessageRouter returning a fixed next hop. When
// panicMsg is set, GetNextHop panics with it.
type staticRouter struct {
	nextHop  MsgNotifySvc
	panicMsg string

	called      int
	lastReplyTo AddrId
	lastTo      AddrId
}

func (r *staticRouter) GetNextHop(replyToAddr, toAddr AddrId) MsgNotifySvc {
	r.called++
	r.lastReplyTo, r.lastTo = replyToAddr, toAddr
	if r.panicMsg != "" {
		panic(r.panicMsg)
	}
	return r.nextHop
}

var (
	hubSender    = AddrId{AddressFamily: MsgNotifyAddrFamilyService, Address: "exam-tracker"}
	hubRecipient = AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: "user@example.com"}
)

func hubTestMsg(id string) Msg {
	return Msg{Id: id, Created: 1735689600000, Title: "t", Level: MessageLevelCommon, Text: "hello"}
}

func TestServiceMessageHub_AcceptedFamilies(t *testing.T) {
	hub := NewServiceMessageHub(&staticRouter{}, "")

	senders := hub.GetAcceptedSenderAddressFamilies()
	if len(senders) != 1 || senders[0] != MsgNotifyAddrFamilyService {
		t.Errorf("sender families = %v, want [%s]", senders, MsgNotifyAddrFamilyService)
	}

	want := map[MsgNotifyAddrFamily]bool{
		MsgNotifyAddrFamilyConsole: true,
		MsgNotifyAddrFamilyEmail:   true,
		MsgNotifyAddrFamilyService: true,
	}
	recipients := hub.GetAcceptedRecipientAddressFamilies()
	if len(recipients) != len(want) {
		t.Fatalf("recipient families = %v, want %d entries", recipients, len(want))
	}
	for _, f := range recipients {
		if !want[f] {
			t.Errorf("unexpected recipient family %q", f)
		}
		delete(want, f)
	}
	for f := range want {
		t.Errorf("missing recipient family %q", f)
	}
}

func TestServiceMessageHub_RoutesToNextHop(t *testing.T) {
	next := &recordingSvc{}
	router := &staticRouter{nextHop: next}
	hub := NewServiceMessageHub(router, "")

	// The exam report server is a known source, so its message is routed.
	from := reportServerSender("taker@example.com")
	wantTo := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: "taker@example.com"}
	msg := hubTestMsg("hub-1")
	if err := hub.Send(context.Background(), from, hubRecipient, msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if router.called != 1 {
		t.Fatalf("GetNextHop called %d times, want 1", router.called)
	}
	if !router.lastReplyTo.AddrEqual(from) || !router.lastTo.AddrEqual(wantTo) {
		t.Errorf("GetNextHop args = (%v, %v), want (%v, %v)", router.lastReplyTo, router.lastTo, from, wantTo)
	}
	if len(next.sent) != 1 {
		t.Fatalf("next hop received %d messages, want 1", len(next.sent))
	}
	got := next.sent[0]
	if !got.replyTo.AddrEqual(from) || !got.to.AddrEqual(wantTo) {
		t.Errorf("next hop received from=%v to=%v, want from=%v to=%v", got.replyTo, got.to, from, wantTo)
	}
	if got.msg.Id != msg.Id || got.msg.Text != msg.Text || got.msg.Created != msg.Created {
		t.Errorf("next hop received msg %+v, want %+v", got.msg, msg)
	}
}

func TestServiceMessageHub_ExamReportServerMessage_DerivesSender(t *testing.T) {
	t.Run("sysadmin configured: the sender becomes the sysadmin email address", func(t *testing.T) {
		next := &recordingSvc{}
		router := &staticRouter{nextHop: next}
		hub := NewServiceMessageHub(router, "sysadmin@example.com")

		from := reportServerSender("taker@example.com")
		if err := hub.Send(context.Background(), from, hubRecipient, hubTestMsg("hub-derive-sender")); err != nil {
			t.Fatalf("Send: %v", err)
		}

		wantSender := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: "sysadmin@example.com"}
		if !router.lastReplyTo.AddrEqual(wantSender) {
			t.Errorf("GetNextHop replyTo = %v, want %v", router.lastReplyTo, wantSender)
		}
		if len(next.sent) != 1 {
			t.Fatalf("next hop received %d messages, want 1", len(next.sent))
		}
		if !next.sent[0].replyTo.AddrEqual(wantSender) {
			t.Errorf("next hop received replyTo = %v, want %v", next.sent[0].replyTo, wantSender)
		}
	})

	t.Run("no sysadmin: the original sender passes through unchanged", func(t *testing.T) {
		next := &recordingSvc{}
		router := &staticRouter{nextHop: next}
		hub := NewServiceMessageHub(router, "")

		from := reportServerSender("taker@example.com")
		if err := hub.Send(context.Background(), from, hubRecipient, hubTestMsg("hub-derive-sender-none")); err != nil {
			t.Fatalf("Send: %v", err)
		}
		if !router.lastReplyTo.AddrEqual(from) {
			t.Errorf("GetNextHop replyTo = %v, want the original sender %v", router.lastReplyTo, from)
		}
	})
}

// sessionServerSender builds a service sender address labeled as the exam
// session server.
func sessionServerSender() AddrId {
	return AddrId{
		AddressFamily: MsgNotifyAddrFamilyService,
		Address:       WellKnownAddrServiceOnMemoryExamSessionServer,
		Tags: AssociationsList{
			MakeLabelKey(WellKnownLabelKeyMsgSource, WellKnownLabelValueExamSessionServer),
		},
	}
}

// TestServiceMessageHub_ExamSessionServerMessage_SilentlyDropped confirms
// that exam-session-started notifications are dropped: only exam-completion
// notifications are routed, so session starts no longer reach a mailbox.
func TestServiceMessageHub_ExamSessionServerMessage_SilentlyDropped(t *testing.T) {
	for _, sysadmin := range []string{"", "sysadmin@example.com"} {
		next := &recordingSvc{}
		router := &staticRouter{nextHop: next}
		hub := NewServiceMessageHub(router, sysadmin)

		if err := hub.Send(context.Background(), sessionServerSender(), hubRecipient, hubTestMsg("hub-session-drop")); err != nil {
			t.Fatalf("Send(sysadmin=%q) = %v, want nil: session-started events are dropped silently", sysadmin, err)
		}
		if router.called != 0 {
			t.Errorf("sysadmin=%q: GetNextHop called %d times, want 0 for a session-started event", sysadmin, router.called)
		}
		if len(next.sent) != 0 {
			t.Errorf("sysadmin=%q: next hop received %d messages, want 0", sysadmin, len(next.sent))
		}
	}
}

// TestServiceMessageHub_NonCompletionReportServerMessage_SilentlyDropped
// confirms that exam report server messages that are not exam-completion
// notifications (a report deletion, or no event label at all) are dropped
// silently.
func TestServiceMessageHub_NonCompletionReportServerMessage_SilentlyDropped(t *testing.T) {
	senders := map[string]AddrId{
		"report deleted event": {
			AddressFamily: MsgNotifyAddrFamilyService,
			Address:       WellKnownAddrServiceOnMemoryExamTrackingServer,
			Tags: AssociationsList{
				MakeLabelKey(WellKnownLabelKeyMsgSource, WellKnownLabelValueExamReportServer),
				MakeLabelKey(WellKnownLabelKeyExamEvent, WellKnownLabelValueExamReportDeleted),
				MakeLabelKey(WellKnownLabelKeyExamTakerExmail, "taker@example.com"),
			},
		},
		"no event label": {
			AddressFamily: MsgNotifyAddrFamilyService,
			Address:       WellKnownAddrServiceOnMemoryExamTrackingServer,
			Tags: AssociationsList{
				MakeLabelKey(WellKnownLabelKeyMsgSource, WellKnownLabelValueExamReportServer),
				MakeLabelKey(WellKnownLabelKeyExamTakerExmail, "taker@example.com"),
			},
		},
	}
	for name, from := range senders {
		t.Run(name, func(t *testing.T) {
			next := &recordingSvc{}
			router := &staticRouter{nextHop: next}
			hub := NewServiceMessageHub(router, "sysadmin@example.com")

			if err := hub.Send(context.Background(), from, hubRecipient, hubTestMsg("hub-noncompletion")); err != nil {
				t.Fatalf("Send = %v, want nil: non-completion messages are dropped silently", err)
			}
			if router.called != 0 {
				t.Errorf("GetNextHop called %d times, want 0", router.called)
			}
			if len(next.sent) != 0 {
				t.Errorf("next hop received %d messages, want 0", len(next.sent))
			}
		})
	}
}

// TestServiceMessageHub_RequiresExplicitMailingConsent confirms that the
// exam taker's mailbox is used only with an explicit
// examreport_mail_consent=true label: any other value (or none at all)
// derives the console stdout destination instead of a mailbox.
func TestServiceMessageHub_RequiresExplicitMailingConsent(t *testing.T) {
	consoleDst := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout}

	senderWithConsent := func(consent *string) AddrId {
		tags := AssociationsList{
			MakeLabelKey(WellKnownLabelKeyMsgSource, WellKnownLabelValueExamReportServer),
			MakeLabelKey(WellKnownLabelKeyExamEvent, WellKnownLabelValueExamCompleted),
			MakeLabelKey(WellKnownLabelKeyExamTakerExmail, "taker@example.com"),
		}
		if consent != nil {
			tags = append(tags, MakeLabelKey(WellKnownLabelKeyExamReportMailConsent, *consent))
		}
		return AddrId{
			AddressFamily: MsgNotifyAddrFamilyService,
			Address:       WellKnownAddrServiceOnMemoryExamTrackingServer,
			Tags:          tags,
		}
	}

	strptr := func(s string) *string { return &s }

	tests := []struct {
		name    string
		consent *string // nil: label absent
		wantTo  AddrId
	}{
		{"explicit true reaches the exam taker", strptr("true"), AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: "taker@example.com"}},
		{"explicit false falls back to the console", strptr("false"), consoleDst},
		{"empty value falls back to the console", strptr(""), consoleDst},
		{"non-boolean value falls back to the console", strptr("yes"), consoleDst},
		{"capitalized True is not consent", strptr("True"), consoleDst},
		{"missing label falls back to the console", nil, consoleDst},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			next := &recordingSvc{}
			router := &staticRouter{nextHop: next}
			hub := NewServiceMessageHub(router, "sysadmin@example.com")

			if err := hub.Send(context.Background(), senderWithConsent(tc.consent), hubRecipient, hubTestMsg("hub-consent")); err != nil {
				t.Fatalf("Send = %v, want nil", err)
			}
			if len(next.sent) != 1 {
				t.Fatalf("next hop received %d messages, want 1", len(next.sent))
			}
			if got := next.sent[0].to; !got.AddrEqual(tc.wantTo) {
				t.Errorf("delivered to %v, want %v", got, tc.wantTo)
			}
		})
	}
}

func TestServiceMessageHub_UnknownSourceSilentlyDropped(t *testing.T) {
	sources := map[string]AddrId{
		// A service sender with no msg_source label at all.
		"unlabeled source": hubSender,
		// A sender labeled with a source the hub has no handling defined for.
		"unknown labeled source": {
			AddressFamily: MsgNotifyAddrFamilyService,
			Address:       "some-other-service",
			Tags: AssociationsList{
				MakeLabelKey(WellKnownLabelKeyMsgSource, "some_unknown_source"),
			},
		},
	}
	for name, from := range sources {
		t.Run(name, func(t *testing.T) {
			next := &recordingSvc{}
			router := &staticRouter{nextHop: next}
			hub := NewServiceMessageHub(router, "sysadmin@example.com")

			if err := hub.Send(context.Background(), from, hubRecipient, hubTestMsg("hub-unknown")); err != nil {
				t.Fatalf("Send = %v, want nil: unknown sources are dropped silently", err)
			}
			if router.called != 0 {
				t.Errorf("GetNextHop called %d times, want 0 for an unknown source", router.called)
			}
			if len(next.sent) != 0 {
				t.Errorf("next hop received %d messages, want 0", len(next.sent))
			}
		})
	}
}

func TestServiceMessageHub_RejectsNonServiceSender(t *testing.T) {
	router := &staticRouter{nextHop: &recordingSvc{}}
	hub := NewServiceMessageHub(router, "")

	err := hub.Send(context.Background(),
		AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: "user@example.com"},
		hubRecipient, hubTestMsg("hub-rej"))
	if err == nil {
		t.Fatal("Send from a non-service sender succeeded, want error")
	}
	if want := `unsupported sender address family "email"`; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to contain %q", err, want)
	}
	if router.called != 0 {
		t.Errorf("GetNextHop called %d times, want 0 for a rejected sender", router.called)
	}
}

func TestServiceMessageHub_NoNextHopLogsAndDrops(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	hub := NewServiceMessageHub(&staticRouter{nextHop: nil}, "")
	if err := hub.Send(context.Background(), reportServerSender("taker@example.com"), hubRecipient, hubTestMsg("hub-drop-1")); err != nil {
		t.Fatalf("Send = %v, want nil: a missing next hop is logged, not an error", err)
	}
	if !strings.Contains(buf.String(), "hub-drop-1") {
		t.Errorf("log output = %q, want it to mention the dropped message id", buf.String())
	}
}

func TestServiceMessageHub_NilRouterLogsAndDrops(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	hub := NewServiceMessageHub(nil, "")
	if err := hub.Send(context.Background(), hubSender, hubRecipient, hubTestMsg("hub-drop-2")); err != nil {
		t.Fatalf("Send = %v, want nil: a missing router is logged, not an error", err)
	}
	if !strings.Contains(buf.String(), "hub-drop-2") {
		t.Errorf("log output = %q, want it to mention the dropped message id", buf.String())
	}
}

func TestServiceMessageHub_AreYou(t *testing.T) {
	// The hub answers yes for any query, regardless of the router.
	hubs := map[string]*ServiceMessageHub{
		"with router":    NewServiceMessageHub(&staticRouter{nextHop: &recordingSvc{}}, ""),
		"nil next hop":   NewServiceMessageHub(&staticRouter{nextHop: nil}, ""),
		"without router": NewServiceMessageHub(nil, ""),
	}
	for name, hub := range hubs {
		for _, addr := range []AddrId{hubSender, hubRecipient, {}} {
			if !hub.AreYou(addr) {
				t.Errorf("%s: AreYou(%v) = false, want true for any query", name, addr)
			}
		}
	}
}

func TestServiceMessageHub_RouterPanicPropagates(t *testing.T) {
	hub := NewServiceMessageHub(&staticRouter{panicMsg: "router exploded"}, "")

	from := reportServerSender("taker@example.com")
	defer func() {
		r := recover()
		if r == nil {
			t.Error("Send swallowed the router panic, want it to propagate")
		} else if r != "router exploded" {
			t.Errorf("recovered panic = %v, want %q", r, "router exploded")
		}
	}()
	_ = hub.Send(context.Background(), from, hubRecipient, hubTestMsg("hub-panic"))
}

// reportServerSender builds a service sender address labeled as the exam
// report server emitting an exam-completion notification, with the given exam
// taker email label value and explicit mailing consent.
func reportServerSender(takerEmail string) AddrId {
	return AddrId{
		AddressFamily: MsgNotifyAddrFamilyService,
		Address:       WellKnownAddrServiceOnMemoryExamTrackingServer,
		Tags: AssociationsList{
			MakeLabelKey(WellKnownLabelKeyMsgSource, WellKnownLabelValueExamReportServer),
			MakeLabelKey(WellKnownLabelKeyExamEvent, WellKnownLabelValueExamCompleted),
			MakeLabelKey(WellKnownLabelKeyExamTakerExmail, takerEmail),
			MakeLabelKey(WellKnownLabelKeyExamReportMailConsent, "true"),
		},
	}
}

func TestServiceMessageHub_ExamReportServerMessage_DerivesDestinationFromExamTakerEmail(t *testing.T) {
	next := &recordingSvc{}
	router := &staticRouter{nextHop: next}
	hub := NewServiceMessageHub(router, "sysadmin@example.com")

	from := reportServerSender("taker@example.com")
	if err := hub.Send(context.Background(), from, hubRecipient, hubTestMsg("hub-derive")); err != nil {
		t.Fatalf("Send: %v", err)
	}

	wantTo := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: "taker@example.com"}
	if !router.lastTo.AddrEqual(wantTo) {
		t.Errorf("GetNextHop to = %v, want %v", router.lastTo, wantTo)
	}
	if len(next.sent) != 1 {
		t.Fatalf("next hop received %d messages, want 1", len(next.sent))
	}
	if !next.sent[0].to.AddrEqual(wantTo) {
		t.Errorf("next hop received to = %v, want %v", next.sent[0].to, wantTo)
	}
}

// TestServiceMessageHub_ExamReportServerMessage_FallsBackToConsole confirms
// that a completion notification whose exam taker email label is empty is
// derived to the console destination, not to a mailbox — even with explicit
// mailing consent.
func TestServiceMessageHub_ExamReportServerMessage_FallsBackToConsole(t *testing.T) {
	next := &recordingSvc{}
	router := &staticRouter{nextHop: next}
	hub := NewServiceMessageHub(router, "sysadmin@example.com")

	// The exam taker email label is present but empty.
	from := reportServerSender("")
	if err := hub.Send(context.Background(), from, hubRecipient, hubTestMsg("hub-console")); err != nil {
		t.Fatalf("Send: %v", err)
	}

	wantTo := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout}
	if !router.lastTo.AddrEqual(wantTo) {
		t.Errorf("GetNextHop to = %v, want %v", router.lastTo, wantTo)
	}
}

// TestServiceMessageHub_ConsoleFallbackNeedsNoSysadmin confirms that the
// console fallback destination always resolves, even with no sysadmin email
// configured, so the router is still consulted and the next hop still
// receives the message.
func TestServiceMessageHub_ConsoleFallbackNeedsNoSysadmin(t *testing.T) {
	next := &recordingSvc{}
	router := &staticRouter{nextHop: next}
	hub := NewServiceMessageHub(router, "") // no sysadmin email

	from := reportServerSender("")
	if err := hub.Send(context.Background(), from, hubRecipient, hubTestMsg("hub-console-nosysadmin")); err != nil {
		t.Fatalf("Send: %v", err)
	}

	wantTo := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout}
	if !router.lastTo.AddrEqual(wantTo) {
		t.Errorf("GetNextHop to = %v, want %v", router.lastTo, wantTo)
	}
	if len(next.sent) != 1 || !next.sent[0].to.AddrEqual(wantTo) {
		t.Errorf("next hop received %+v, want one message to %v", next.sent, wantTo)
	}
}

func TestServiceMessageHub_NextHopErrorPropagates(t *testing.T) {
	want := errors.New("delivery failed")
	hub := NewServiceMessageHub(&staticRouter{nextHop: &recordingSvc{err: want}}, "")

	err := hub.Send(context.Background(), reportServerSender("taker@example.com"), hubRecipient, hubTestMsg("hub-err"))
	if !errors.Is(err, want) {
		t.Errorf("Send error = %v, want %v", err, want)
	}
}
