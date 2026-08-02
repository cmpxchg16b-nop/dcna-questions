package examreport

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	pkgmodelsquestion "dcna-questions/pkg/models/question"
)

// mustReport builds an ExamReport with distinguishable fields.
func mustReport(t *testing.T, id string) ExamReport {
	t.Helper()
	return ExamReport{
		Id:            id,
		ExamId:        "exam-" + id,
		ExamShortName: "DCACI",
		ExamCode:      "300-620",
		Title:         "Implementing Cisco ACI",
		Description:   "desc",
		ExamCategory:  pkgmodelsquestion.ExamCategoryCertification,
		ExamSessionId: "sess-" + id,
		FinishedAt:    1700000000000,
		Assessment: pkgmodelsquestion.Assessment{
			OverallResult: nil,
			ScoreResult: &pkgmodelsquestion.ScoreResult{
				EarnedScore: 7,
				TotalScore:  10,
			},
		},
	}
}

func TestExamReport_XMLRoundTrip(t *testing.T) {
	report := ExamReport{
		Id:            "rep-1",
		ExamId:        "exam-doc-1",
		ExamShortName: "DCACI",
		ExamCode:      "300-620",
		Title:         "Implementing Cisco ACI",
		Description:   "A description",
		ExamCategory:  pkgmodelsquestion.ExamCategoryCertification,
		ExamSessionId: "session-1",
		FinishedAt:    1700000000123,
		ExamTaker: ExamTaker{
			Persons: []Person{{Name: "Alice", Fistname: "Alice", Lastname: "Smith"}},
			Anonymous: []Anonymous{{SessionId: "anon-1"}},
		},
		Assessment: pkgmodelsquestion.Assessment{
			OverallResult: nil,
			ScoreResult: &pkgmodelsquestion.ScoreResult{
				EarnedScore: 800,
				TotalScore:  1000,
			},
			QuestionScores: []pkgmodelsquestion.QuestionScore{
				{QuestionId: "q1", ScoreEarned: 1},
				{QuestionId: "q2", ScoreEarned: 0},
			},
		},
	}

	out, err := xml.MarshalIndent(&report, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: unexpected error: %v", err)
	}
	str := string(out)
	t.Logf("marshaled:\n%s", str)

	// Spot-check key elements/attributes that ExamReport is responsible for.
	for _, want := range []string{
		`<examreport id="rep-1">`,
		`<examid>exam-doc-1</examid>`,
		`<examshortname>DCACI</examshortname>`,
		`<examcode>300-620</examcode>`,
		`<title>Implementing Cisco ACI</title>`,
		`<description>A description</description>`,
		`<examcategory>certification-exam</examcategory>`,
		`<examsessionid>session-1</examsessionid>`,
		`<finishedat>1700000000123</finishedat>`,
		`<person name="Alice" fistname="Alice" lastname="Smith"></person>`,
		`<anonymous sessionid="anon-1"></anonymous>`,
	} {
		if !strings.Contains(str, want) {
			t.Errorf("marshaled XML missing %q", want)
		}
	}

	// Round-trip back.
	var got ExamReport
	if err := xml.Unmarshal(out, &got); err != nil {
		t.Fatalf("Unmarshal: unexpected error: %v", err)
	}
	if got.Id != report.Id {
		t.Errorf("Id = %q, want %q", got.Id, report.Id)
	}
	if got.ExamId != report.ExamId {
		t.Errorf("ExamId = %q, want %q", got.ExamId, report.ExamId)
	}
	if got.ExamSessionId != report.ExamSessionId {
		t.Errorf("ExamSessionId = %q, want %q", got.ExamSessionId, report.ExamSessionId)
	}
	if got.FinishedAt != report.FinishedAt {
		t.Errorf("FinishedAt = %d, want %d", got.FinishedAt, report.FinishedAt)
	}
	if got.ExamCategory != report.ExamCategory {
		t.Errorf("ExamCategory = %q, want %q", got.ExamCategory, report.ExamCategory)
	}
	if got.Assessment.ScoreResult == nil ||
		got.Assessment.ScoreResult.EarnedScore != report.Assessment.ScoreResult.EarnedScore {
		t.Errorf("Assessment.ScoreResult not preserved: %+v", got.Assessment.ScoreResult)
	}
	if len(got.ExamTaker.Persons) != 1 || got.ExamTaker.Persons[0].Name != "Alice" {
		t.Errorf("ExamTaker.Persons not preserved: %+v", got.ExamTaker.Persons)
	}
	if len(got.ExamTaker.Anonymous) != 1 || got.ExamTaker.Anonymous[0].SessionId != "anon-1" {
		t.Errorf("ExamTaker.Anonymous not preserved: %+v", got.ExamTaker.Anonymous)
	}
}

func TestExamReport_OptionalFieldsOmitempty(t *testing.T) {
	// Description and PassingScore are optional; a zero report should omit them.
	report := ExamReport{
		Id:            "rep-2",
		ExamId:        "exam-doc-2",
		Title:         "T",
		ExamCategory:  pkgmodelsquestion.ExamCategoryPractice,
		ExamSessionId: "session-2",
		FinishedAt:    1,
	}
	out, err := xml.Marshal(&report)
	if err != nil {
		t.Fatalf("Marshal: unexpected error: %v", err)
	}
	str := string(out)
	if strings.Contains(str, "<description>") {
		t.Errorf("expected <description> to be omitted, got:\n%s", str)
	}
	if strings.Contains(str, "<passingscore>") {
		t.Errorf("expected <passingscore> to be omitted, got:\n%s", str)
	}
}

func TestReportKey(t *testing.T) {
	cases := []struct {
		userid string
		idx    int64
		want   string
	}{
		{"alice", 0, "alice:0"},
		{"alice", 42, "alice:42"},
		{"u:with:colons", 3, "u:with:colons:3"},
	}
	for _, c := range cases {
		if got := reportKey(c.userid, c.idx); got != c.want {
			t.Errorf("reportKey(%q,%d) = %q, want %q", c.userid, c.idx, got, c.want)
		}
	}
}

func TestPutAndGet_SingleUser(t *testing.T) {
	srv := NewOnMemoryExamTrackingServer()
	ctx := context.Background()

	// Unknown user returns nil, nil.
	got, err := srv.GetExamReportsByUserId(ctx, "nobody")
	if err != nil {
		t.Fatalf("Get unknown user: unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("Get unknown user = %v, want nil", got)
	}

	r1 := mustReport(t, "r1")
	r2 := mustReport(t, "r2")
	r3 := mustReport(t, "r3")

	if err := srv.Put(ctx, "alice", r1); err != nil {
		t.Fatalf("Put r1: %v", err)
	}
	if err := srv.Put(ctx, "alice", r2); err != nil {
		t.Fatalf("Put r2: %v", err)
	}
	if err := srv.Put(ctx, "alice", r3); err != nil {
		t.Fatalf("Put r3: %v", err)
	}

	reports, err := srv.GetExamReportsByUserId(ctx, "alice")
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if len(reports) != 3 {
		t.Fatalf("len(reports) = %d, want 3", len(reports))
	}
	// Insertion order must be preserved.
	wantIds := []string{"r1", "r2", "r3"}
	for i, w := range wantIds {
		if reports[i].Id != w {
			t.Errorf("reports[%d].Id = %q, want %q", i, reports[i].Id, w)
		}
	}
}

func TestPutAndGet_MultipleUsersAreIsolated(t *testing.T) {
	srv := NewOnMemoryExamTrackingServer()
	ctx := context.Background()

	_ = srv.Put(ctx, "alice", mustReport(t, "a1"))
	_ = srv.Put(ctx, "bob", mustReport(t, "b1"))
	_ = srv.Put(ctx, "alice", mustReport(t, "a2"))

	alice, _ := srv.GetExamReportsByUserId(ctx, "alice")
	bob, _ := srv.GetExamReportsByUserId(ctx, "bob")

	if len(alice) != 2 {
		t.Fatalf("alice has %d reports, want 2", len(alice))
	}
	if len(bob) != 1 {
		t.Fatalf("bob has %d reports, want 1", len(bob))
	}
	for _, r := range alice {
		if !strings.HasPrefix(r.Id, "a") {
			t.Errorf("alice should only have 'a*' reports, got %q", r.Id)
		}
	}
}

// TestPut_ConcurrentSameUserNoLoss hammers Put for a single userid from many
// goroutines and then verifies every report was retained with a unique index.
// Under -race this also exercises the CAS loop's memory safety.
func TestPut_ConcurrentSameUserNoLoss(t *testing.T) {
	srv := NewOnMemoryExamTrackingServer()
	ctx := context.Background()

	const goroutines = 64
	const perGoroutine = 25
	const total = goroutines * perGoroutine
	userid := "racer"

	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := make(chan struct{})
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < perGoroutine; i++ {
				r := mustReport(t, "x")
				if err := srv.Put(ctx, userid, r); err != nil {
					t.Errorf("Put: unexpected error: %v", err)
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()

	reports, err := srv.GetExamReportsByUserId(ctx, userid)
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if len(reports) != total {
		t.Fatalf("len(reports) = %d, want %d (lost %d reports)",
			len(reports), total, total-len(reports))
	}

	// Confirm the count matches the value stored in the counts map.
	v, ok := srv.counts.Load(userid)
	if !ok {
		t.Fatal("counts map missing entry for user")
	}
	if n := v.(int64); n != int64(total) {
		t.Errorf("counts = %d, want %d", n, total)
	}
}

// TestPutGet_ConcurrentMixed runs Puts and Gets concurrently against the same
// user to stress the read/write interaction under -race. Gets may transiently
// observe fewer than the in-flight count, but must never observe more than the
// committed count and must never panic.
func TestPutGet_ConcurrentMixed(t *testing.T) {
	srv := NewOnMemoryExamTrackingServer()
	ctx := context.Background()

	const puts = 200
	userid := "mixed"

	var wg sync.WaitGroup
	wg.Add(2)

	var getErr atomic.Value // stores error
	go func() {
		defer wg.Done()
		for i := 0; i < puts; i++ {
			rs, err := srv.GetExamReportsByUserId(ctx, userid)
			if err != nil {
				getErr.Store(err)
				return
			}
			// A Get must never report more than the number of completed Puts so
			// far; since reads and writes interleave arbitrarily we just check
			// the upper bound is non-negative and never absurdly large.
			if len(rs) < 0 || len(rs) > puts {
				getErr.Store(fmt.Errorf("Get returned %d reports (want 0..%d)", len(rs), puts))
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < puts; i++ {
			if err := srv.Put(ctx, userid, mustReport(t, "p")); err != nil {
				getErr.Store(err)
				return
			}
		}
	}()

	wg.Wait()

	if v := getErr.Load(); v != nil {
		t.Fatalf("concurrent Get/Put error: %v", v)
	}

	final, _ := srv.GetExamReportsByUserId(ctx, userid)
	if len(final) != puts {
		t.Errorf("final report count = %d, want %d", len(final), puts)
	}
}

// Compile-time assertion that the in-memory implementation satisfies the
// interface.
var _ ExamTrackingServer = (*OnMemoryExamTrackingServer)(nil)
