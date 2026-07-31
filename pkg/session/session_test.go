package session

import (
	"testing"
)

func newSession() *Session {
	return &Session{id: "s1"}
}

func TestListExamsEmpty(t *testing.T) {
	s := newSession()
	if got := s.ListExams(); len(got) != 0 {
		t.Fatalf("expected empty list, got %v", got)
	}
}

func TestGetExamByIdSeedsWhenAbsent(t *testing.T) {
	s := newSession()
	// LoadOrStore: an absent id is seeded with a fresh ExamSession, never nil.
	got := s.GetUserExamSessionById("nope")
	if got == nil {
		t.Fatal("expected non-nil seeded ExamSession")
	}
	if got.ExamId != "nope" {
		t.Fatalf("expected seeded ExamId=nope, got %q", got.ExamId)
	}
	if ids := s.ListExams(); len(ids) != 1 || ids[0] != "nope" {
		t.Fatalf("expected seeded id to be listed, got %v", ids)
	}
}

func TestGetExamByIdReturnsExisting(t *testing.T) {
	s := newSession()
	// Creation goes through GetExamById (LoadOrStore); UpdateExam only does a
	// CAS against the snapshot the caller last observed.
	cur := s.GetUserExamSessionById("e1")
	existing := UserExamSession{ExamId: "e1", TblVer: 7}
	if !s.UpdateUserExamSession("e1", *cur, existing) {
		t.Fatal("update failed")
	}
	got := s.GetUserExamSessionById("e1")
	if got == nil || got.TblVer != 7 {
		t.Fatalf("expected existing TblVer=7, got %+v", got)
	}
}

func TestUpdateExamCASConflict(t *testing.T) {
	s := newSession()
	// Seed via GetExamById, then advance through CAS. The client always CASes
	// against the snapshot it last read; the server never guarantees success.
	seed := s.GetUserExamSessionById("e2")
	v1 := UserExamSession{ExamId: "e2", TblVer: 1}
	if !s.UpdateUserExamSession("e2", *seed, v1) {
		t.Fatal("CAS from seed to v1 failed")
	}
	v2 := UserExamSession{ExamId: "e2", TblVer: 2}
	if !s.UpdateUserExamSession("e2", v1, v2) {
		t.Fatal("expected CAS from v1 to v2 to succeed")
	}
	// A stale client still holding v1 must fail to swap.
	stale := UserExamSession{ExamId: "e2", TblVer: 3}
	if s.UpdateUserExamSession("e2", v1, stale) {
		t.Fatal("expected CAS with stale old value to fail")
	}
	if got := s.GetUserExamSessionById("e2"); got.TblVer != 2 {
		t.Fatalf("expected unchanged TblVer=2, got %d", got.TblVer)
	}
}
