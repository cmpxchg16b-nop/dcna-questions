package msgnotify

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

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
