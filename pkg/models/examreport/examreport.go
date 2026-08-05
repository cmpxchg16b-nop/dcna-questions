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
	"strconv"
	"sync"

	pkgmodelsquestion "dcna-questions/pkg/models/question"
)

// ErrExamTrackingNotFound is returned by DeleteExamTracking when the user has
// no exam report with the given id.
var ErrExamTrackingNotFound = errors.New("examreport: exam report not found")

// Person is one named <person> within an <examtaker>: a real exam candidate
// identified by full name. Fistname is spelled as in the XSD attribute.
type Person struct {
	Name     string `xml:"name,attr" json:"name"`
	Fistname string `xml:"fistname,attr" json:"fistname,omitempty"`
	Lastname string `xml:"lastname,attr" json:"lastname,omitempty"`
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
	// Put stores an exam report for the given userid.
	Put(ctx context.Context, userid string, examReport ExamReport) error

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
}

// NewOnMemoryExamTrackingServer returns a ready-to-use OnMemoryExamTrackingServer.
func NewOnMemoryExamTrackingServer() *OnMemoryExamTrackingServer {
	return &OnMemoryExamTrackingServer{}
}

// Put stores examReport under userid. It is safe for concurrent use: Put claims
// a unique per-user index by compare-and-swapping the count, so concurrent Puts
// for the same userid never collide on the same key.
func (s *OnMemoryExamTrackingServer) Put(ctx context.Context, userid string, examReport ExamReport) error {
	s.counts.LoadOrStore(userid, int64(0))
	for {
		cur, _ := s.counts.Load(userid)
		idx := cur.(int64)
		// Atomically claim idx by advancing the count only if it is still idx.
		if s.counts.CompareAndSwap(userid, idx, idx+1) {
			// idx is now ours: safe to store the report at this index.
			s.reports.Store(reportKey(userid, idx), examReport)
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
		if rv, ok := s.reports.Load(key); ok && rv.(ExamReport).Id == examReportId {
			s.reports.Delete(key)
			return nil
		}
	}
	return ErrExamTrackingNotFound
}

// reportKey builds the synthesized reports-map key "{userid}:{index}".
func reportKey(userid string, idx int64) string {
	return userid + ":" + strconv.FormatInt(idx, 10)
}
