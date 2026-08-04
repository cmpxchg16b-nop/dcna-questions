package examserver

import (
	"context"
	"path/filepath"
	"testing"

	pkgmodelquestions "dcna-questions/pkg/models/question"

	"dcna-questions/pkg/models/examreport"
)

// TestStartNewExamSession_WalksAllQuestions loads the real exam1.xml and confirms that
// a fresh session presents every question in its question collection, in order.
func TestStartNewExamSession_WalksAllQuestions(t *testing.T) {
	exam, err := pkgmodelquestions.NewFileExamLoader().LoadFile(filepath.Join("..", "..", "..", "exam1.xml"))
	if err != nil {
		t.Fatalf("load exam: %v", err)
	}

	srv := NewOnMemoryExamServer(examreport.NewOnMemoryExamTrackingServer())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	// exam1.xml is a certification exam, so it must not be created seekable;
	// GetNextQuestion walks questions sequentially regardless of seekability.
	examId, err := srv.StartNewExamSession(ctx, exam, "user-1", 0, nil)
	if err != nil {
		t.Fatalf("StartNewExamSession: %v", err)
	}

	var seen []string
	var cursor *QuestionCursor
	for {
		q, next, err := srv.GetNextQuestion(ctx, examId, "user-1", cursor)
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

	// exam1.xml has one collection with 4 questions (ids 1, 4, 6, 7).
	if len(seen) != 4 {
		t.Fatalf("expected 4 questions, got %d (%v)", len(seen), seen)
	}
	for i, want := range []string{"1", "4", "6", "7"} {
		if seen[i] != want {
			t.Errorf("question %d: got id %q, want %q", i, seen[i], want)
		}
	}
}

// TestStartNewExamSession_EmptyExam verifies that an exam with no questions is rejected.
func TestStartNewExamSession_EmptyExam(t *testing.T) {
	srv := NewOnMemoryExamServer(examreport.NewOnMemoryExamTrackingServer())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	if _, err := srv.StartNewExamSession(ctx, nil, "user-1", 0, nil); err != errEmptyExam {
		t.Fatalf("expected errEmptyExam for nil exam, got %v", err)
	}

	empty := &pkgmodelquestions.Exam{Id: "x"}
	if _, err := srv.StartNewExamSession(ctx, empty, "user-1", 0, nil); err != errEmptyExam {
		t.Fatalf("expected errEmptyExam for empty exam, got %v", err)
	}
}

// TestStartNewExamSession_CertificationRejectsSeekable confirms that a
// certification exam cannot be started with ExamOptionSeekable: the candidate
// must answer questions in the fixed order they are served.
func TestStartNewExamSession_CertificationRejectsSeekable(t *testing.T) {
	mkQ := func(id string) pkgmodelquestions.Question {
		return pkgmodelquestions.Question{Id: id, Type: pkgmodelquestions.QuestionTypeSingleChoice}
	}
	certExam := &pkgmodelquestions.Exam{
		Id:           "cert",
		ExamCategory: pkgmodelquestions.ExamCategoryCertification,
		QuestionSet: pkgmodelquestions.QuestionSet{
			QuestionCollections: []pkgmodelquestions.QuestionCollection{
				{Questions: []pkgmodelquestions.Question{mkQ("q1")}},
			},
		},
	}

	srv := NewOnMemoryExamServer(examreport.NewOnMemoryExamTrackingServer())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	if _, err := srv.StartNewExamSession(ctx, certExam, "user-1", ExamOptionSeekable, nil); err != errSeekableNotAllowed {
		t.Fatalf("certification exam with ExamOptionSeekable = %v, want errSeekableNotAllowed", err)
	}

	// The same certification exam without the seekable bit starts fine.
	if _, err := srv.StartNewExamSession(ctx, certExam, "user-1", 0, nil); err != nil {
		t.Fatalf("certification exam without seekable: %v", err)
	}
}

// TestStartNewExamSession_PracticeAllowsSeekable confirms that a practice exam
// may be started seekable and that seeking then works end to end.
func TestStartNewExamSession_PracticeAllowsSeekable(t *testing.T) {
	mkQ := func(id string) pkgmodelquestions.Question {
		return pkgmodelquestions.Question{Id: id, Type: pkgmodelquestions.QuestionTypeSingleChoice}
	}
	practiceExam := &pkgmodelquestions.Exam{
		Id:           "practice",
		ExamCategory: pkgmodelquestions.ExamCategoryPractice,
		QuestionSet: pkgmodelquestions.QuestionSet{
			QuestionCollections: []pkgmodelquestions.QuestionCollection{
				{Questions: []pkgmodelquestions.Question{mkQ("1"), mkQ("2"), mkQ("3")}},
			},
		},
	}

	srv := NewOnMemoryExamServer(examreport.NewOnMemoryExamTrackingServer())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	examId, err := srv.StartNewExamSession(ctx, practiceExam, "user-1", ExamOptionSeekable, nil)
	if err != nil {
		t.Fatalf("practice exam with ExamOptionSeekable: %v", err)
	}

	// Walk to the first question to obtain a cursor, then seek back to it.
	q, cursor, err := srv.GetNextQuestion(ctx, examId, "user-1", nil)
	if err != nil || q == nil {
		t.Fatalf("GetNextQuestion: q=%v err=%v", q, err)
	}
	if _, err := srv.SeekCursorTo(ctx, examId, "user-1", cursor, 0); err != nil {
		t.Errorf("SeekCursorTo on practice exam = %v, want nil", err)
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

	srv := NewOnMemoryExamServer(examreport.NewOnMemoryExamTrackingServer())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	examId, err := srv.StartNewExamSession(ctx, exam, "user-1", ExamOptionRandomQuestionColl|ExamOptionSeekable, nil)
	if err != nil {
		t.Fatalf("StartNewExamSession: %v", err)
	}

	// All questions in the chosen collection share their leading letter; the
	// flattened path would mix letters.
	var ids []string
	var cursor *QuestionCursor
	for {
		q, next, err := srv.GetNextQuestion(ctx, examId, "user-1", cursor)
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

	srv := NewOnMemoryExamServer(examreport.NewOnMemoryExamTrackingServer())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	examId, err := srv.StartNewExamSession(ctx, exam, "user-1", ExamOptionSeekable, nil)
	if err != nil {
		t.Fatalf("StartNewExamSession: %v", err)
	}

	var n int
	var cursor *QuestionCursor
	for {
		q, next, err := srv.GetNextQuestion(ctx, examId, "user-1", cursor)
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

// TestGetExamSessionById_CurrentQuestionIndex confirms that a freshly started
// session reports CurrentQuestionIndex == -1 (no question fetched yet), and
// that the index advances to 0, 1, ... as the owner calls GetNextQuestion. It
// also checks that a non-owner cannot retrieve the session and that a missing
// session is reported as an error.
func TestGetExamSessionById_CurrentQuestionIndex(t *testing.T) {
	mkQ := func(id string) pkgmodelquestions.Question {
		return pkgmodelquestions.Question{Id: id, Type: pkgmodelquestions.QuestionTypeSingleChoice}
	}
	exam := &pkgmodelquestions.Exam{
		Id: "index-tracking",
		QuestionSet: pkgmodelquestions.QuestionSet{
			QuestionCollections: []pkgmodelquestions.QuestionCollection{
				{Questions: []pkgmodelquestions.Question{mkQ("1"), mkQ("2"), mkQ("3")}},
			},
		},
	}

	srv := NewOnMemoryExamServer(examreport.NewOnMemoryExamTrackingServer())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	examId, err := srv.StartNewExamSession(ctx, exam, "user-1", ExamOptionSeekable, nil)
	if err != nil {
		t.Fatalf("StartNewExamSession: %v", err)
	}

	// Before any GetNextQuestion, the current index is -1.
	ex, err := srv.GetExamSessionById(ctx, examId, "user-1")
	if err != nil {
		t.Fatalf("GetExamSessionById: %v", err)
	}
	if ex.CurrentQuestionIndex != -1 {
		t.Fatalf("initial CurrentQuestionIndex = %d, want -1", ex.CurrentQuestionIndex)
	}
	if ex.CurrentQuestion != nil {
		t.Fatalf("initial CurrentQuestion = %v, want nil", ex.CurrentQuestion)
	}
	if ex.Id != examId {
		t.Errorf("excerpt.Id = %q, want %q", ex.Id, examId)
	}

	// Each GetNextQuestion should advance the reported index by one.
	var cursor *QuestionCursor
	for want := 0; want < 3; want++ {
		q, next, err := srv.GetNextQuestion(ctx, examId, "user-1", cursor)
		if err != nil || q == nil {
			t.Fatalf("GetNextQuestion(%d): q=%v err=%v", want, q, err)
		}
		cursor = next

		ex, err := srv.GetExamSessionById(ctx, examId, "user-1")
		if err != nil {
			t.Fatalf("GetExamSessionById after fetch %d: %v", want, err)
		}
		if ex.CurrentQuestionIndex != want {
			t.Errorf("after fetch %d, CurrentQuestionIndex = %d, want %d", want, ex.CurrentQuestionIndex, want)
		}
		if ex.CurrentQuestion == nil {
			t.Errorf("after fetch %d, CurrentQuestion = nil, want non-nil", want)
		} else if ex.CurrentQuestion.Id != q.Id {
			t.Errorf("after fetch %d, CurrentQuestion.Id = %q, want %q", want, ex.CurrentQuestion.Id, q.Id)
		}
	}

	// A non-owner must be rejected.
	if _, err := srv.GetExamSessionById(ctx, examId, "user-2"); err != errNotOwner {
		t.Errorf("GetExamSessionById by non-owner = %v, want errNotOwner", err)
	}

	// A missing session must be rejected.
	if _, err := srv.GetExamSessionById(ctx, "does-not-exist", "user-1"); err != errExamNotFound {
		t.Errorf("GetExamSessionById for missing session = %v, want errExamNotFound", err)
	}
}

// hasCorrectAnswer reports whether q carries any correct-answer content.
func hasCorrectAnswer(q *pkgmodelquestions.Question) bool {
	ca := q.CorrectAnswer
	return len(ca.Options) > 0 || len(ca.Combinations) > 0 || len(ca.ConnectionSolutions) > 0
}

// TestGetNextQuestion_StripsCorrectAnswer verifies that served questions never
// carry the correct answer — neither from GetNextQuestion nor from the session
// excerpt's CurrentQuestion — while the grader's internal answer key is
// unaffected and the practice-exam assessment remains the one place the
// correct answer is revealed.
func TestGetNextQuestion_StripsCorrectAnswer(t *testing.T) {
	exam := &pkgmodelquestions.Exam{
		Id:           "strip",
		ExamCategory: pkgmodelquestions.ExamCategoryPractice,
		QuestionSet: pkgmodelquestions.QuestionSet{
			QuestionCollections: []pkgmodelquestions.QuestionCollection{
				{Questions: []pkgmodelquestions.Question{
					singleChoice("sc", 1, "1"),
					dndQuestion("dnd", 1, pkgmodelquestions.ConnectionSolution{
						RequiredUniqueConnections: 1,
						Connects:                  []pkgmodelquestions.Connect{{Src: "a", Dst: "b"}},
					}),
				}},
			},
		},
	}

	srv := NewOnMemoryExamServer(examreport.NewOnMemoryExamTrackingServer())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	examId, err := srv.StartNewExamSession(ctx, exam, "user-1", 0, nil)
	if err != nil {
		t.Fatalf("StartNewExamSession: %v", err)
	}

	// Every served question must be stripped, including the one echoed back as
	// the session excerpt's CurrentQuestion.
	var cursor *QuestionCursor
	for i := 0; i < 2; i++ {
		q, next, err := srv.GetNextQuestion(ctx, examId, "user-1", cursor)
		if err != nil || q == nil {
			t.Fatalf("GetNextQuestion(%d): q=%v err=%v", i, q, err)
		}
		if hasCorrectAnswer(q) {
			t.Errorf("GetNextQuestion(%d) leaked correct answer for %q", i, q.Id)
		}
		cursor = next

		ex, err := srv.GetExamSessionById(ctx, examId, "user-1")
		if err != nil {
			t.Fatalf("GetExamSessionById after fetch %d: %v", i, err)
		}
		if ex.CurrentQuestion == nil || ex.CurrentQuestion.Id != q.Id {
			t.Fatalf("after fetch %d, CurrentQuestion = %v, want Id %q", i, ex.CurrentQuestion, q.Id)
		}
		if hasCorrectAnswer(ex.CurrentQuestion) {
			t.Errorf("CurrentQuestion leaked correct answer for %q", q.Id)
		}
	}

	// Grading still sees the answer key: both answers correct -> full score,
	// and the practice-exam assessment reveals the correct answers.
	assessment, err := srv.SubmitAnswer(ctx, examId, "user-1", examAnswer(
		answer("sc", "1"),
		pkgmodelquestions.Answer{QuestionId: "dnd", Connections: connects([2]string{"a", "b"})},
	), false)
	if err != nil {
		t.Fatalf("SubmitAnswer: %v", err)
	}
	if assessment.ScoreResult.EarnedScore != 2 {
		t.Errorf("earned = %g, want 2", assessment.ScoreResult.EarnedScore)
	}
	if len(assessment.Questions) != 2 {
		t.Fatalf("assessment embedded %d questions, want 2", len(assessment.Questions))
	}
	for _, q := range assessment.Questions {
		if !hasCorrectAnswer(&q) {
			t.Errorf("assessment question %q lost its correct answer", q.Id)
		}
	}
}

// TestOwnershipEnforcement confirms that a user cannot operate on an exam
// session that belongs to another user: EndExamSession, GetNextQuestion,
// SeekCursorTo, and SubmitAnswer all reject a non-owner caller, while the
// owner is unaffected.
func TestOwnershipEnforcement(t *testing.T) {
	mkQ := func(id string) pkgmodelquestions.Question {
		return pkgmodelquestions.Question{Id: id, Type: pkgmodelquestions.QuestionTypeSingleChoice}
	}
	exam := &pkgmodelquestions.Exam{
		Id: "ownership",
		QuestionSet: pkgmodelquestions.QuestionSet{
			QuestionCollections: []pkgmodelquestions.QuestionCollection{
				{Questions: []pkgmodelquestions.Question{mkQ("q1")}},
			},
		},
	}

	srv := NewOnMemoryExamServer(examreport.NewOnMemoryExamTrackingServer())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	examId, err := srv.StartNewExamSession(ctx, exam, "user-1", ExamOptionSeekable, nil)
	if err != nil {
		t.Fatalf("StartNewExamSession: %v", err)
	}

	// A different user must be blocked from every operation on the session.
	if err := srv.EndExamSession(ctx, examId, "user-2"); err != errNotOwner {
		t.Errorf("EndExamSession by non-owner = %v, want errNotOwner", err)
	}
	if _, _, err := srv.GetNextQuestion(ctx, examId, "user-2", nil); err != errNotOwner {
		t.Errorf("GetNextQuestion by non-owner = %v, want errNotOwner", err)
	}
	if _, err := srv.SeekCursorTo(ctx, examId, "user-2", nil, 0); err != errNotOwner {
		t.Errorf("SeekCursorTo by non-owner = %v, want errNotOwner", err)
	}
	if _, err := srv.SubmitAnswer(ctx, examId, "user-2", &pkgmodelquestions.ExamAnswer{}, false); err != errNotOwner {
		t.Errorf("SubmitAnswer by non-owner = %v, want errNotOwner", err)
	}
	if _, err := srv.GetExamSessionById(ctx, examId, "user-2"); err != errNotOwner {
		t.Errorf("GetExamSessionById by non-owner = %v, want errNotOwner", err)
	}

	// The owner can still use their own session.
	if q, _, err := srv.GetNextQuestion(ctx, examId, "user-1", nil); err != nil || q == nil {
		t.Fatalf("GetNextQuestion by owner: q=%v err=%v", q, err)
	}
}

// TestStartNewExamSession_AcceptQuestionTypes loads the real exam1.xml and
// confirms that acceptQuestionTypes restricts the served questions to the
// listed types, that the session excerpt reports the filtered question count,
// and that a filter matching nothing fails like an empty exam.
func TestStartNewExamSession_AcceptQuestionTypes(t *testing.T) {
	exam, err := pkgmodelquestions.NewFileExamLoader().LoadFile(filepath.Join("..", "..", "..", "exam1.xml"))
	if err != nil {
		t.Fatalf("load exam: %v", err)
	}

	srv := NewOnMemoryExamServer(examreport.NewOnMemoryExamTrackingServer())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	// exam1.xml: questions 1, 4, 6 are single-choice and 7 is drag-and-drop.
	// Accepting only the choice types must skip the drag-and-drop question.
	accept := []pkgmodelquestions.QuestionType{
		pkgmodelquestions.QuestionTypeSingleChoice,
		pkgmodelquestions.QuestionTypeMultipleChoice,
	}
	examId, err := srv.StartNewExamSession(ctx, exam, "user-1", 0, accept)
	if err != nil {
		t.Fatalf("StartNewExamSession: %v", err)
	}

	var seen []string
	var cursor *QuestionCursor
	for {
		q, next, err := srv.GetNextQuestion(ctx, examId, "user-1", cursor)
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

	want := []string{"1", "4", "6"}
	if len(seen) != len(want) {
		t.Fatalf("expected %d questions, got %d (%v)", len(want), len(seen), seen)
	}
	for i, id := range want {
		if seen[i] != id {
			t.Errorf("question %d: got id %q, want %q", i, seen[i], id)
		}
	}

	// The session excerpt must describe the filtered question set, not the
	// exam document's full collection, so clients can bound navigation.
	ex, err := srv.GetExamSessionById(ctx, examId, "user-1")
	if err != nil {
		t.Fatalf("GetExamSessionById: %v", err)
	}
	if ex.ExamExcerpt.NumQuestions != len(want) {
		t.Errorf("excerpt NumQuestions = %d, want %d", ex.ExamExcerpt.NumQuestions, len(want))
	}

	// A filter matching no question at all is rejected like an empty exam.
	if _, err := srv.StartNewExamSession(ctx, exam, "user-1", 0, []pkgmodelquestions.QuestionType{"essay"}); err != errEmptyExam {
		t.Errorf("StartNewExamSession with unmatched filter = %v, want errEmptyExam", err)
	}
}
