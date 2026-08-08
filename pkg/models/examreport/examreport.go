// Package examreport defines the data model for an exam report: the full
// report produced after an exam taker finishes an exam session.
//
// It mirrors the <examreport> and <examtaker> elements defined in exam.xsd.
// Assessment-related types (overall result, scores, assessment) are reused from
// the question package.
package examreport

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"dcna-questions/pkg/models/msgnotify"
	pkgmodelsquestion "dcna-questions/pkg/models/question"

	"github.com/google/uuid"
)

// ErrExamTrackingNotFound is returned by DeleteExamTracking when the user has
// no exam report with the given id.
var ErrExamTrackingNotFound = errors.New("examreport: exam report not found")

// Person is one named <person> within an <examtaker>: a real exam candidate
// identified by full name. Fistname is spelled as in the XSD attribute. Email
// is the candidate's email address; it lets the exam report server deliver
// the report to the candidate when they consented to mailing.
type Person struct {
	Name     string `xml:"name,attr" json:"name"`
	Fistname string `xml:"fistname,attr" json:"fistname,omitempty"`
	Lastname string `xml:"lastname,attr" json:"lastname,omitempty"`
	Email    string `xml:"email,attr" json:"email,omitempty"`
}

// Anonymous is one <anonymous> entry within an <examtaker>: an unidentified
// exam taker tracked only by session id.
type Anonymous struct {
	SessionId string `xml:"sessionid,attr" json:"sessionId"`
}

// ExamTaker is the <examtaker> element: the list of persons and/or anonymous
// sessions who took the exam. Either may be empty.
type ExamTaker struct {
	XMLName   xml.Name    `xml:"examtaker" json:"-"`
	Persons   []Person    `xml:"person" json:"persons,omitempty"`
	Anonymous []Anonymous `xml:"anonymous" json:"anonymous,omitempty"`
}

// ExamReport is the <examreport> element: a full report sent to the exam
// assessment tracking server after an exam taker has finished the exam session.
type ExamReport struct {
	XMLName xml.Name `xml:"examreport" json:"-"`

	// Id is the id of the exam report; it has to be globally unique, not the id
	// of the exam document, nor the id of the exam session.
	Id string `xml:"id,attr" json:"id"`

	// ExamTaker is the person or anonymous session that took the exam.
	ExamTaker ExamTaker `xml:"examtaker" json:"examTaker"`

	// ExamId is the exam document id, not the exam session id.
	ExamId string `xml:"examid" json:"examId"`

	// ExamShortName is the short name copied from the origin exam document.
	ExamShortName string `xml:"examshortname" json:"examShortName,omitempty"`

	// ExamCode is the code copied from the origin exam document.
	ExamCode string `xml:"examcode" json:"examCode,omitempty"`

	// Title is the title of the exam.
	Title string `xml:"title" json:"title"`

	// Description is the description of the exam. Optional.
	Description string `xml:"description,omitempty" json:"description,omitempty"`

	// PassingScore is the mandated passing score of the exam, copied directly
	// from the exam element.
	PassingScore *float32 `xml:"passingscore" json:"passingScore,omitempty"`

	// ExamCategory is copied directly from the origin exam document too.
	ExamCategory pkgmodelsquestion.ExamCategory `xml:"examcategory" json:"examCategory"`

	// ExamSessionId is the id of the exam session which the exam taker was in.
	ExamSessionId string `xml:"examsessionid" json:"examSessionId"`

	// FinishedAt is the millisecond-resolution unix timestamp when the exam
	// session was finished by the exam taker.
	FinishedAt int64 `xml:"finishedat" json:"finishedAt"`

	// Assessment contains the grade and the score that was achieved by the
	// exam taker.
	Assessment pkgmodelsquestion.Assessment `xml:"assessment" json:"assessment"`
}

// ExamTrackingServer is the server that persists and retrieves exam reports.
// Reports are keyed by the exam taker (userid), so the history of a user's
// finished exam sessions can be looked up via GetExamReportsByUserId.
type ExamTrackingServer interface {
	// Put stores an exam report for the given userid. mailingConsent is the
	// exam taker's consent to the exam report being emailed to the exam
	// taker's email address; it is carried as a label on the notification
	// emitted for the stored report, so downstream messaging can act on it.
	Put(ctx context.Context, userid string, examReport ExamReport, mailingConsent bool) error

	// GetExamReportsByUserId returns all exam reports recorded for userid.
	GetExamReportsByUserId(ctx context.Context, userid string) ([]ExamReport, error)

	// DeleteExamTracking removes the exam report identified by examReportId
	// from userid's reports. It returns ErrExamTrackingNotFound when the user
	// has no report with that id.
	DeleteExamTracking(ctx context.Context, userid string, examReportId string) error
}

// OnMemoryExamTrackingServer is an in-memory, lock-free implementation of
// ExamTrackingServer. It is safe for concurrent use by multiple goroutines.
//
// It uses two sync.Maps:
//
//   - reports maps the synthesized key "{userid}:{index}" to an ExamReport;
//   - counts  maps userid to an int64 holding the number of reports stored for
//     that user.
//
// Put advances a user's count via sync.Map.CompareAndSwap to atomically claim a
// unique index: it reads the current count c, then CAS-advances it to c+1; only
// when the CAS succeeds is index c known to be safe to write. Because the count
// is monotonically increased only by Put (it never decreases), there is no ABA
// problem: each Put observes a strictly increasing index and no report is ever
// overwritten or lost, with no mutex required.
//
// DeleteExamTracking removes a report's map entry but never decrements the
// count, so deletion leaves a hole in the user's index space: Put's invariants
// above are untouched, and Get skips the hole exactly as it skips the
// in-flight window of a concurrent Put.
type OnMemoryExamTrackingServer struct {
	// reports maps "{userid}:{index}" to ExamReport.
	reports sync.Map
	// counts maps userid to int64, the number of reports for that user.
	counts sync.Map

	// notifiers are the messaging notification services that are asked to
	// deliver a notification when an exam report is stored or deleted. It may
	// be empty.
	notifiers []msgnotify.MsgNotifySvc
}

// NewOnMemoryExamTrackingServer returns a ready-to-use OnMemoryExamTrackingServer.
// notifiers is the list of messaging notification services notified when an
// exam report is stored (Put) or deleted (DeleteExamTracking); it may be nil
// or empty, in which case no notifications are sent.
func NewOnMemoryExamTrackingServer(notifiers []msgnotify.MsgNotifySvc) *OnMemoryExamTrackingServer {
	return &OnMemoryExamTrackingServer{notifiers: notifiers}
}

// notificationSender is the reply-to address the OnMemoryExamTrackingServer
// uses for the notifications it sends.
var notificationSender = msgnotify.AddrId{
	AddressFamily: msgnotify.MsgNotifyAddrFamilyService,
	Address:       msgnotify.WellKnownAddrServiceOnMemoryExamTrackingServer,
}

// notificationRecipient is the address notifications are addressed to. The
// tracking server does not know — and never cares — who the final recipient
// is: its job is only to hand the message to the next hop, the notifiers that
// claim the address (AreYou == true); the lifelong fate of the message is the
// next hop's concern.
var notificationRecipient = msgnotify.AddrId{
	AddressFamily: msgnotify.MsgNotifyAddrFamilyService,
	Address:       "",
}

// notify delivers a notification through every notifier that accepts both the
// sender and the recipient address families and claims the recipient address
// when probed with AreYou, so unsupported services are skipped before Send is
// attempted. Delivery is best-effort: Send errors are logged and never fail
// the tracking operation.
//
// mailingConsent is the exam taker's consent to the exam report being emailed
// to the exam taker's email address; it is carried on the message as the
// WellKnownLabelKeyExamReportMailConsent label so downstream messaging can act
// on it. Operations that have no consent decision (DeleteExamTracking) pass
// false.
func (s *OnMemoryExamTrackingServer) notify(ctx context.Context, userid string, report ExamReport, mailingConsent bool, title string, level msgnotify.MessageLevel, text string) {
	// The sender carries the message tags: the message source, the exam
	// taker's subject id, the overall result of the exam assessment, the
	// mailing consent, and the exam taker labels lifted from the report's
	// first person (left empty when the exam taker is anonymous).
	overallResult := ""
	if report.Assessment.OverallResult != nil {
		overallResult = string(*report.Assessment.OverallResult)
	}
	var exmail, username, firstName, lastName string
	if len(report.ExamTaker.Persons) > 0 {
		p := report.ExamTaker.Persons[0]
		exmail, username, firstName, lastName = p.Email, p.Name, p.Fistname, p.Lastname
	}
	sender := notificationSender
	sender.Tags = msgnotify.AssociationsList{
		msgnotify.MakeLabelKey(msgnotify.WellKnownLabelKeyMsgSource, msgnotify.WellKnownLabelValueExamReportServer),
		msgnotify.MakeLabelKey(msgnotify.WellKnownLabelKeyExamTakerSubjectId, userid),
		msgnotify.MakeLabelKey(msgnotify.WellKnownLabelKeyExamOverallResult, overallResult),
		msgnotify.MakeLabelKey(msgnotify.WellKnownLabelKeyExamReportMailConsent, strconv.FormatBool(mailingConsent)),
		msgnotify.MakeLabelKey(msgnotify.WellKnownLabelKeyExamTakerExmail, exmail),
		msgnotify.MakeLabelKey(msgnotify.WellKnownLabelKeyExamTakerUsername, username),
		msgnotify.MakeLabelKey(msgnotify.WellKnownLabelKeyExamTakerFirstName, firstName),
		msgnotify.MakeLabelKey(msgnotify.WellKnownLabelKeyExamTakerLastName, lastName),
	}

	msg := msgnotify.Msg{
		Id:      uuid.NewString(),
		Created: time.Now().UnixMilli(),
		Title:   title,
		Level:   level,
		Text:    text,
	}
	for _, svc := range s.notifiers {
		if !acceptsAddrFamily(svc.GetAcceptedSenderAddressFamilies(), sender.AddressFamily) {
			continue
		}
		if !acceptsAddrFamily(svc.GetAcceptedRecipientAddressFamilies(), notificationRecipient.AddressFamily) {
			continue
		}
		// Confirm with the service that it is the recipient before emitting.
		if !svc.AreYou(notificationRecipient) {
			continue
		}
		if err := svc.Send(ctx, sender, notificationRecipient, msg); err != nil {
			slog.WarnContext(ctx, "examreport: notification delivery failed",
				"messageId", msg.Id, "to", notificationRecipient, "error", err)
		}
	}
}

// acceptsAddrFamily reports whether family is present in families.
func acceptsAddrFamily(families []msgnotify.MsgNotifyAddrFamily, family msgnotify.MsgNotifyAddrFamily) bool {
	for _, f := range families {
		if f == family {
			return true
		}
	}
	return false
}

// Put stores examReport under userid. It is safe for concurrent use: Put claims
// a unique per-user index by compare-and-swapping the count, so concurrent Puts
// for the same userid never collide on the same key.
func (s *OnMemoryExamTrackingServer) Put(ctx context.Context, userid string, examReport ExamReport, mailingConsent bool) error {
	s.counts.LoadOrStore(userid, int64(0))
	for {
		cur, _ := s.counts.Load(userid)
		idx := cur.(int64)
		// Atomically claim idx by advancing the count only if it is still idx.
		if s.counts.CompareAndSwap(userid, idx, idx+1) {
			// idx is now ours: safe to store the report at this index.
			s.reports.Store(reportKey(userid, idx), examReport)
			s.notify(ctx, userid, examReport, mailingConsent, "Exam session completed", msgnotify.MessageLevelCommon,
				fmt.Sprintf("User %s completed exam session %s; exam report %s recorded.", userid, examReport.ExamSessionId, examReport.Id))
			return nil
		}
	}
}

// GetExamReportsByUserId returns all exam reports stored for userid in the order
// they were Put, or nil if the user has none. The returned slice is independent
// of the stored state and safe to use without further synchronization.
//
// Note: the count is advanced before a report is stored, so under a concurrent
// Put Get may observe a count that includes an in-flight report; such
// not-yet-stored entries are skipped and will appear on a subsequent call. No
// report is ever lost.
func (s *OnMemoryExamTrackingServer) GetExamReportsByUserId(ctx context.Context, userid string) ([]ExamReport, error) {
	v, ok := s.counts.Load(userid)
	if !ok {
		return nil, nil
	}
	n := v.(int64)
	out := make([]ExamReport, 0, n)
	for i := int64(0); i < n; i++ {
		if rv, ok := s.reports.Load(reportKey(userid, i)); ok {
			out = append(out, rv.(ExamReport))
		}
	}
	return out, nil
}

// DeleteExamTracking removes the report with the given id from userid's
// reports, or returns ErrExamTrackingNotFound when the user has no such
// report. It is safe for concurrent use.
//
// The scan tolerates concurrent Puts: a slot whose count was already claimed
// but whose report is not yet stored simply fails the Load and is skipped,
// exactly as in Get. Deletion never decrements the count, so indexes stay
// monotonic and a deleted index is never reused; because each index is
// written at most once, the Load-then-Delete pair can only ever remove the
// report it observed — no CAS or mutex is needed. Two concurrent deletes of
// the same id may both return nil; the net effect is one deletion, so the
// operation is safely idempotent.
func (s *OnMemoryExamTrackingServer) DeleteExamTracking(ctx context.Context, userid string, examReportId string) error {
	v, ok := s.counts.Load(userid)
	if !ok {
		return ErrExamTrackingNotFound
	}
	n := v.(int64)
	for i := int64(0); i < n; i++ {
		key := reportKey(userid, i)
		rv, ok := s.reports.Load(key)
		if !ok {
			continue
		}
		report := rv.(ExamReport)
		if report.Id != examReportId {
			continue
		}
		s.reports.Delete(key)
		s.notify(ctx, userid, report, false, "Exam report deleted", msgnotify.MessageLevelImportant,
			fmt.Sprintf("User %s deleted exam report %s.", userid, examReportId))
		return nil
	}
	return ErrExamTrackingNotFound
}

// reportKey builds the synthesized reports-map key "{userid}:{index}".
func reportKey(userid string, idx int64) string {
	return userid + ":" + strconv.FormatInt(idx, 10)
}
