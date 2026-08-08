package msgnotify

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

// SimpleConsoleMessagingService is a MsgNotifySvc that writes messages to the
// well-known console addresses /dev/stdout and /dev/stderr. It only accepts
// destination addresses in the console address family; the reply-to address
// may belong to any family. Attachments are ignored: only the plaintext Text
// is written.
type SimpleConsoleMessagingService struct{}

var _ MsgNotifySvc = SimpleConsoleMessagingService{}

// AreYou reports whether addrId is one of the well-known console addresses
// this service writes to: /dev/stdout or /dev/stderr in the console family.
func (SimpleConsoleMessagingService) AreYou(addrId AddrId) bool {
	return addrId.AddressFamily == MsgNotifyAddrFamilyConsole &&
		(addrId.Address == WellKnownAddrConsoleStdout || addrId.Address == WellKnownAddrConsoleStderr)
}

// GetAcceptedSenderAddressFamilies returns all known address families: the
// service tolerates a reply-to address of any family.
func (SimpleConsoleMessagingService) GetAcceptedSenderAddressFamilies() []MsgNotifyAddrFamily {
	return []MsgNotifyAddrFamily{
		MsgNotifyAddrFamilyConsole,
		MsgNotifyAddrFamilyEmail,
		MsgNotifyAddrFamilyService,
	}
}

// GetAcceptedRecipientAddressFamilies returns only the console family: the
// service can only deliver to console addresses.
func (SimpleConsoleMessagingService) GetAcceptedRecipientAddressFamilies() []MsgNotifyAddrFamily {
	return []MsgNotifyAddrFamily{MsgNotifyAddrFamilyConsole}
}

func (SimpleConsoleMessagingService) Send(ctx context.Context, replyTo AddrId, to AddrId, msg Msg) error {
	if to.AddressFamily != MsgNotifyAddrFamilyConsole {
		return fmt.Errorf("msgnotify: unsupported destination address family %q", to.AddressFamily)
	}

	var w io.Writer
	switch to.Address {
	case WellKnownAddrConsoleStdout:
		w = os.Stdout
	case WellKnownAddrConsoleStderr:
		w = os.Stderr
	default:
		return fmt.Errorf("msgnotify: unknown console address %q", to.Address)
	}

	_, err := fmt.Fprintf(w, "At %s, Level: %d, From: %s:%s Message: %s\n",
		time.UnixMilli(msg.Created).Format(time.RFC3339),
		msg.Level,
		replyTo.AddressFamily, replyTo.Address,
		msg.Text,
	)
	return err
}
