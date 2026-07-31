package exam

import (
	"context"
	"path/filepath"
	"testing"

	pkgmodelquestions "dcna-questions/pkg/models/question"
)

// TestStartNewExamSession_WalksAllQuestions loads the real exam1.xml and confirms that
// a fresh session presents every question in its question collection, in order.
func TestStartNewExamSession_WalksAllQuestions(t *testing.T) {
	exam, err := pkgmodelquestions.NewFileExamLoader().LoadFile(filepath.Join("..", "..", "..", "exam1.xml"))
	if err != nil {
		t.Fatalf("load exam: %v", err)
	}

	srv := NewOnMemoryExamServer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	examId, err := srv.StartNewExamSession(ctx, exam, "user-1", ExamOptionSeekable)
	if err != nil {
		t.Fatalf("StartNewExamSession: %v", err)
	}

	var seen []string
	var cursor *QuestionCursor
	for {
		q, next, err := srv.GetNextQuestion(ctx, examId, cursor)
		if err != nil {
			t.Fatalf("GetNextQuestion: %v", err)
		}
		if q == nil {
			break // no more questions
		}
		seen = append(seen, q.Id)
		cursor = next
		if cursor == nil {
			break
		}
	}

	// exam1.xml has one collection with 5 questions (ids 1, 2, 3, 4, 5).
	if len(seen) != 5 {
		t.Fatalf("expected 5 questions, got %d (%v)", len(seen), seen)
	}
	for i, want := range []string{"1", "2", "3", "4", "5"} {
		if seen[i] != want {
			t.Errorf("question %d: got id %q, want %q", i, seen[i], want)
		}
	}
}

// TestStartNewExamSession_EmptyExam verifies that an exam with no questions is rejected.
func TestStartNewExamSession_EmptyExam(t *testing.T) {
	srv := NewOnMemoryExamServer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	if _, err := srv.StartNewExamSession(ctx, nil, "user-1", 0); err != errEmptyExam {
		t.Fatalf("expected errEmptyExam for nil exam, got %v", err)
	}

	empty := &pkgmodelquestions.Exam{Id: "x"}
	if _, err := srv.StartNewExamSession(ctx, empty, "user-1", 0); err != errEmptyExam {
		t.Fatalf("expected errEmptyExam for empty exam, got %v", err)
	}
}

// TestStartNewExamSession_RandomCollPicksOneCollection builds a synthetic exam with
// several collections and, with ExamOptionRandomQuestionColl set, confirms the
// session is backed by exactly one collection's questions (a subset), not the
// flattened set.
func TestStartNewExamSession_RandomCollPicksOneCollection(t *testing.T) {
	mkQ := func(id string) pkgmodelquestions.Question {
		return pkgmodelquestions.Question{Id: id, Type: pkgmodelquestions.QuestionTypeSingleChoice}
	}
	exam := &pkgmodelquestions.Exam{
		Id: "synthetic",
		QuestionSet: pkgmodelquestions.QuestionSet{
			QuestionCollections: []pkgmodelquestions.QuestionCollection{
				{Questions: []pkgmodelquestions.Question{mkQ("a1"), mkQ("a2")}},
				{Questions: []pkgmodelquestions.Question{mkQ("b1"), mkQ("b2")}},
				{Questions: []pkgmodelquestions.Question{mkQ("c1"), mkQ("c2")}},
			},
		},
	}

	srv := NewOnMemoryExamServer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	examId, err := srv.StartNewExamSession(ctx, exam, "user-1", ExamOptionRandomQuestionColl|ExamOptionSeekable)
	if err != nil {
		t.Fatalf("StartNewExamSession: %v", err)
	}

	// All questions in the chosen collection share their leading letter; the
	// flattened path would mix letters.
	var ids []string
	var cursor *QuestionCursor
	for {
		q, next, err := srv.GetNextQuestion(ctx, examId, cursor)
		if err != nil {
			t.Fatalf("GetNextQuestion: %v", err)
		}
		if q == nil {
			break
		}
		ids = append(ids, q.Id)
		cursor = next
		if cursor == nil {
			break
		}
	}

	if len(ids) != 2 {
		t.Fatalf("random collection should yield exactly 2 questions, got %d (%v)", len(ids), ids)
	}
	prefix := ids[0][:1]
	for _, id := range ids {
		if id[:1] != prefix {
			t.Fatalf("questions come from different collections (mixed prefixes): %v", ids)
		}
	}
}

// TestStartNewExamSession_FlattensCollectionsByDefault confirms that without
// ExamOptionRandomQuestionColl, every question across all collections is shown.
func TestStartNewExamSession_FlattensCollectionsByDefault(t *testing.T) {
	mkQ := func(id string) pkgmodelquestions.Question {
		return pkgmodelquestions.Question{Id: id, Type: pkgmodelquestions.QuestionTypeSingleChoice}
	}
	exam := &pkgmodelquestions.Exam{
		Id: "synthetic",
		QuestionSet: pkgmodelquestions.QuestionSet{
			QuestionCollections: []pkgmodelquestions.QuestionCollection{
				{Questions: []pkgmodelquestions.Question{mkQ("a1"), mkQ("a2")}},
				{Questions: []pkgmodelquestions.Question{mkQ("b1"), mkQ("b2")}},
			},
		},
	}

	srv := NewOnMemoryExamServer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	examId, err := srv.StartNewExamSession(ctx, exam, "user-1", ExamOptionSeekable)
	if err != nil {
		t.Fatalf("StartNewExamSession: %v", err)
	}

	var n int
	var cursor *QuestionCursor
	for {
		q, next, err := srv.GetNextQuestion(ctx, examId, cursor)
		if err != nil {
			t.Fatalf("GetNextQuestion: %v", err)
		}
		if q == nil {
			break
		}
		n++
		cursor = next
		if cursor == nil {
			break
		}
	}
	if n != 4 {
		t.Fatalf("expected flattened total of 4 questions, got %d", n)
	}
}
