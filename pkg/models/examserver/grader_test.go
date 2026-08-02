package examserver

import (
	"context"
	"testing"

	pkgmodelquestions "dcna-questions/pkg/models/question"

	"dcna-questions/pkg/models/examreport"
)

// float32Ptr returns a pointer to v; used to build optional passing scores.
func float32Ptr(v float32) *float32 { return &v }

// singleChoice builds a single-choice question worth score points whose correct
// answer is any one of correctOptIds.
func singleChoice(id string, score float32, correctOptIds ...string) pkgmodelquestions.Question {
	return pkgmodelquestions.Question{
		Id:    id,
		Type:  pkgmodelquestions.QuestionTypeSingleChoice,
		Score: score,
		CorrectAnswer: pkgmodelquestions.CorrectAnswer{
			Options: options(correctOptIds...),
		},
	}
}

// multiChoice builds a multiple-choice question worth score points whose correct
// answer is the exact set correctOptIds.
func multiChoice(id string, score float32, correctOptIds ...string) pkgmodelquestions.Question {
	return pkgmodelquestions.Question{
		Id:    id,
		Type:  pkgmodelquestions.QuestionTypeMultipleChoice,
		Score: score,
		CorrectAnswer: pkgmodelquestions.CorrectAnswer{
			Options: options(correctOptIds...),
		},
	}
}

// options builds an Options slice from ids.
func options(ids ...string) pkgmodelquestions.Options {
	opts := make(pkgmodelquestions.Options, len(ids))
	for i, id := range ids {
		opts[i] = pkgmodelquestions.Option{Id: id}
	}
	return opts
}

// answer builds an Answer for qid selecting the given option ids.
func answer(qid string, optIds ...string) pkgmodelquestions.Answer {
	return pkgmodelquestions.Answer{QuestionId: qid, Options: options(optIds...)}
}

// examAnswer builds an ExamAnswer from the given answers.
func examAnswer(answers ...pkgmodelquestions.Answer) *pkgmodelquestions.ExamAnswer {
	return &pkgmodelquestions.ExamAnswer{Answers: answers}
}

// --- SimpleOnMemoryGrader: single-choice ------------------------------------

// TestSimpleOnMemoryGrader_SingleChoice confirms the single-choice membership
// rule: a single submitted option that is among the correct options earns the
// full score; a wrong option earns nothing.
func TestSimpleOnMemoryGrader_SingleChoice(t *testing.T) {
	qc := &pkgmodelquestions.QuestionCollection{
		Questions: []pkgmodelquestions.Question{
			singleChoice("q1", 2, "3", "5"), // accept either 3 or 5
		},
	}
	g := NewSimpleOnMemoryGrader(qc, nil, pkgmodelquestions.ExamCategoryCertification)

	// Correct option "5".
	a := examAnswer(answer("q1", "5"))
	got, err := g.Grade(a)
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if got.ScoreResult.EarnedScore != 2 {
		t.Errorf("correct single-choice: earned = %g, want 2", got.ScoreResult.EarnedScore)
	}
	if got.ScoreResult.TotalScore != 2 {
		t.Errorf("total = %g, want 2", got.ScoreResult.TotalScore)
	}

	// Wrong option "1".
	a = examAnswer(answer("q1", "1"))
	got, _ = g.Grade(a)
	if got.ScoreResult.EarnedScore != 0 {
		t.Errorf("incorrect single-choice: earned = %g, want 0", got.ScoreResult.EarnedScore)
	}
}

// TestSimpleOnMemoryGrader_SingleChoice_RequiresExactlyOneOption verifies that
// submitting zero or more than one option to a single-choice question is wrong,
// even if one of the submitted options is correct.
func TestSimpleOnMemoryGrader_SingleChoice_RequiresExactlyOneOption(t *testing.T) {
	qc := &pkgmodelquestions.QuestionCollection{
		Questions: []pkgmodelquestions.Question{
			singleChoice("q1", 2, "3", "5"),
		},
	}
	g := NewSimpleOnMemoryGrader(qc, nil, pkgmodelquestions.ExamCategoryCertification)

	for _, tc := range []struct {
		name string
		opts []string
	}{
		{"zero options", nil},
		{"two options", []string{"3", "5"}}, // both individually correct, but count != 1
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := g.Grade(examAnswer(answer("q1", tc.opts...)))
			if err != nil {
				t.Fatalf("Grade: %v", err)
			}
			if got.ScoreResult.EarnedScore != 0 {
				t.Errorf("earned = %g, want 0", got.ScoreResult.EarnedScore)
			}
		})
	}
}

// --- SimpleOnMemoryGrader: multiple-choice ----------------------------------

// TestSimpleOnMemoryGrader_MultipleChoice confirms the exact-set rule: the
// submitted option set must match the correct option set exactly, regardless of
// order.
func TestSimpleOnMemoryGrader_MultipleChoice(t *testing.T) {
	qc := &pkgmodelquestions.QuestionCollection{
		Questions: []pkgmodelquestions.Question{
			multiChoice("q1", 3, "1", "4"),
		},
	}
	g := NewSimpleOnMemoryGrader(qc, nil, pkgmodelquestions.ExamCategoryCertification)

	// Correct set, reversed order.
	got, err := g.Grade(examAnswer(answer("q1", "4", "1")))
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if got.ScoreResult.EarnedScore != 3 {
		t.Errorf("correct multiple-choice: earned = %g, want 3", got.ScoreResult.EarnedScore)
	}

	// Partial set: missing "4".
	got, _ = g.Grade(examAnswer(answer("q1", "1")))
	if got.ScoreResult.EarnedScore != 0 {
		t.Errorf("partial multiple-choice: earned = %g, want 0", got.ScoreResult.EarnedScore)
	}

	// Superset: "1", "4", plus wrong "2".
	got, _ = g.Grade(examAnswer(answer("q1", "1", "4", "2")))
	if got.ScoreResult.EarnedScore != 0 {
		t.Errorf("superset multiple-choice: earned = %g, want 0", got.ScoreResult.EarnedScore)
	}
}

// --- SimpleOnMemoryGrader: skipping -----------------------------------------

// TestSimpleOnMemoryGrader_UnsupportedTypeSkipped verifies that a drag-and-drop
// question (the unsupported type here) is silently skipped: it contributes to
// neither an error nor a QuestionScore, though its score still counts toward
// TotalScore.
func TestSimpleOnMemoryGrader_UnsupportedTypeSkipped(t *testing.T) {
	dnd := pkgmodelquestions.Question{
		Id:    "dnd",
		Type:  pkgmodelquestions.QuestionTypeDragAndDrop,
		Score: 5,
	}
	sc := singleChoice("sc", 2, "1")
	qc := &pkgmodelquestions.QuestionCollection{Questions: []pkgmodelquestions.Question{dnd, sc}}
	g := NewSimpleOnMemoryGrader(qc, nil, pkgmodelquestions.ExamCategoryCertification)

	got, err := g.Grade(examAnswer(
		answer("dnd"),                 // unsupported type
		answer("sc", "1"),             // correct single-choice
		answer("does-not-exist", "1"), // unknown question
	))
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if got.ScoreResult.TotalScore != 7 { // 5 + 2
		t.Errorf("total = %g, want 7", got.ScoreResult.TotalScore)
	}
	if got.ScoreResult.EarnedScore != 2 { // only the single-choice
		t.Errorf("earned = %g, want 2", got.ScoreResult.EarnedScore)
	}
	if len(got.QuestionScores) != 1 {
		t.Fatalf("got %d QuestionScores, want 1 (unsupported/unknown skipped)", len(got.QuestionScores))
	}
	if got.QuestionScores[0].QuestionId != "sc" {
		t.Errorf("QuestionScores[0] = %q, want %q", got.QuestionScores[0].QuestionId, "sc")
	}
}

// --- SimpleOnMemoryGrader: OverallResult ------------------------------------

// TestSimpleOnMemoryGrader_OverallResult exercises the pass/immediate decision:
// with no passing score the result is always immediate; otherwise it is pass
// only when earned >= passing score.
func TestSimpleOnMemoryGrader_OverallResult(t *testing.T) {
	qc := &pkgmodelquestions.QuestionCollection{
		Questions: []pkgmodelquestions.Question{singleChoice("q1", 3, "1")},
	}
	correct := examAnswer(answer("q1", "1"))
	wrong := examAnswer(answer("q1", "9"))

	t.Run("no passing score is always immediate", func(t *testing.T) {
		g := NewSimpleOnMemoryGrader(qc, nil, pkgmodelquestions.ExamCategoryCertification)
		got, _ := g.Grade(correct)
		if *got.OverallResult != pkgmodelquestions.OverallResultImmediate {
			t.Errorf("got %q, want immediate", *got.OverallResult)
		}
	})

	t.Run("earned >= passing is pass", func(t *testing.T) {
		g := NewSimpleOnMemoryGrader(qc, float32Ptr(3), pkgmodelquestions.ExamCategoryCertification)
		got, _ := g.Grade(correct)
		if *got.OverallResult != pkgmodelquestions.OverallResultPass {
			t.Errorf("got %q, want pass", *got.OverallResult)
		}
	})

	t.Run("earned < passing is immediate", func(t *testing.T) {
		g := NewSimpleOnMemoryGrader(qc, float32Ptr(3), pkgmodelquestions.ExamCategoryCertification)
		got, _ := g.Grade(wrong)
		if *got.OverallResult != pkgmodelquestions.OverallResultImmediate {
			t.Errorf("got %q, want immediate", *got.OverallResult)
		}
	})
}

// TestSimpleOnMemoryGrader_NilAnswer confirms a nil submission grades cleanly
// to zero earned and immediate result.
func TestSimpleOnMemoryGrader_NilAnswer(t *testing.T) {
	qc := &pkgmodelquestions.QuestionCollection{
		Questions: []pkgmodelquestions.Question{singleChoice("q1", 2, "1")},
	}
	g := NewSimpleOnMemoryGrader(qc, nil, pkgmodelquestions.ExamCategoryCertification)
	got, err := g.Grade(nil)
	if err != nil {
		t.Fatalf("Grade(nil): %v", err)
	}
	if got.ScoreResult.EarnedScore != 0 {
		t.Errorf("earned = %g, want 0", got.ScoreResult.EarnedScore)
	}
	if got.ScoreResult.TotalScore != 2 {
		t.Errorf("total = %g, want 2", got.ScoreResult.TotalScore)
	}
	if len(got.QuestionScores) != 0 {
		t.Errorf("got %d QuestionScores, want 0", len(got.QuestionScores))
	}
}

// --- SimpleOnMemoryGrader: practice-exam question inclusion ----------------

// TestSimpleOnMemoryGrader_PracticeExamIncludesAnsweredQuestions confirms the
// practice-exam rule from the XSD: the original question document (carrying its
// correct answer) is embedded in the assessment, but only for questions the
// candidate actually answered and that were gradeable.
func TestSimpleOnMemoryGrader_PracticeExamIncludesAnsweredQuestions(t *testing.T) {
	sc := singleChoice("sc", 2, "3")
	mc := multiChoice("mc", 4, "1", "2")
	qNotAnswered := singleChoice("skipped", 1, "7")
	dnd := pkgmodelquestions.Question{
		Id:    "dnd",
		Type:  pkgmodelquestions.QuestionTypeDragAndDrop,
		Score: 5,
	}
	qc := &pkgmodelquestions.QuestionCollection{
		Questions: []pkgmodelquestions.Question{sc, mc, qNotAnswered, dnd},
	}
	g := NewSimpleOnMemoryGrader(qc, nil, pkgmodelquestions.ExamCategoryPractice)

	got, err := g.Grade(examAnswer(
		answer("sc", "3"),             // correct
		answer("mc", "9"),             // wrong
		answer("dnd"),                 // unsupported type, skipped
		answer("does-not-exist", "1"), // unknown question, skipped
	))
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}

	// Only the two gradeable, answered questions (sc, mc) are embedded; the
	// skipped/unknown questions and the unanswered "skipped" question are not.
	if len(got.Questions) != 2 {
		t.Fatalf("got %d Questions, want 2", len(got.Questions))
	}
	want := map[string]bool{"sc": false, "mc": false}
	for _, q := range got.Questions {
		if _, ok := want[q.Id]; !ok {
			t.Errorf("unexpected embedded question %q", q.Id)
		}
		want[q.Id] = true
	}
	for id, seen := range want {
		if !seen {
			t.Errorf("question %q not embedded in assessment", id)
		}
	}

	// The embedded single-choice question must carry its correct answer so the
	// candidate can review it.
	for _, q := range got.Questions {
		if q.Id == "sc" {
			if len(q.CorrectAnswer.Options) != 1 || q.CorrectAnswer.Options[0].Id != "3" {
				t.Errorf("embedded sc correctAnswer = %v, want option 3", q.CorrectAnswer.Options)
			}
		}
	}
}

// TestSimpleOnMemoryGrader_CertificationExamOmitsQuestions confirms that a
// certification-exam assessment never embeds the origin questions, regardless
// of how many questions were answered.
func TestSimpleOnMemoryGrader_CertificationExamOmitsQuestions(t *testing.T) {
	qc := &pkgmodelquestions.QuestionCollection{
		Questions: []pkgmodelquestions.Question{singleChoice("q1", 2, "3")},
	}
	g := NewSimpleOnMemoryGrader(qc, nil, pkgmodelquestions.ExamCategoryCertification)

	got, err := g.Grade(examAnswer(answer("q1", "3")))
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if len(got.Questions) != 0 {
		t.Errorf("certification-exam embedded %d questions, want 0", len(got.Questions))
	}
}

// --- SubmitAnswer integration (grading + GetMyAnswer) ------------------------

// newGradingExam builds an exam with one collection of two gradeable questions
// (single- and multiple-choice) plus a drag-and-drop question, and an optional
// passing score.
func newGradingExam(passingScore *float32) *pkgmodelquestions.Exam {
	return &pkgmodelquestions.Exam{
		Id:           "grading",
		ExamCategory: pkgmodelquestions.ExamCategoryCertification,
		PassingScore: passingScore,
		QuestionSet: pkgmodelquestions.QuestionSet{
			QuestionCollections: []pkgmodelquestions.QuestionCollection{
				{Questions: []pkgmodelquestions.Question{
					singleChoice("sc", 2, "3"),
					multiChoice("mc", 4, "1", "2"),
					{Id: "dnd", Type: pkgmodelquestions.QuestionTypeDragAndDrop, Score: 1},
				}},
			},
		},
	}
}

// TestSubmitAnswer_GradesAndStores verifies that SubmitAnswer grades the
// submission, returns an assessment, and—when checkOnly is false—persists the
// answer so that a later GetMyAnswer returns it.
func TestSubmitAnswer_GradesAndStores(t *testing.T) {
	srv := NewOnMemoryExamServer(examreport.NewOnMemoryExamTrackingServer())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	pass := float32Ptr(6.0)
	examId, err := srv.StartNewExamSession(ctx, newGradingExam(pass), "user-1", 0, nil)
	if err != nil {
		t.Fatalf("StartNewExamSession: %v", err)
	}

	// Both gradeable questions correct: earned 2 + 4 = 6 == passing 6 -> pass.
	submission := examAnswer(
		answer("sc", "3"),
		answer("mc", "2", "1"), // order-independent
	)
	got, err := srv.SubmitAnswer(ctx, examId, "user-1", submission, false)
	if err != nil {
		t.Fatalf("SubmitAnswer: %v", err)
	}
	if got.ScoreResult.EarnedScore != 6 {
		t.Errorf("earned = %g, want 6", got.ScoreResult.EarnedScore)
	}
	if got.ScoreResult.TotalScore != 7 { // 2 + 4 + 1 (dnd still counts toward total)
		t.Errorf("total = %g, want 7", got.ScoreResult.TotalScore)
	}
	if got.OverallResult == nil || *got.OverallResult != pkgmodelquestions.OverallResultPass {
		t.Errorf("overall = %v, want pass", got.OverallResult)
	}

	// checkOnly=false persists; GetMyAnswer must return the saved submission.
	stored, err := srv.GetMyAnswer(ctx, examId, "user-1")
	if err != nil {
		t.Fatalf("GetMyAnswer: %v", err)
	}
	if stored == nil {
		t.Fatal("GetMyAnswer returned nil after non-checkOnly submit")
	}
	if len(stored.Answers) != 2 {
		t.Errorf("stored %d answers, want 2", len(stored.Answers))
	}
}

// TestSubmitAnswer_CheckOnly verifies that SubmitAnswer with checkOnly=true
// grades the submission but does not persist it.
func TestSubmitAnswer_CheckOnly(t *testing.T) {
	srv := NewOnMemoryExamServer(examreport.NewOnMemoryExamTrackingServer())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	examId, err := srv.StartNewExamSession(ctx, newGradingExam(nil), "user-1", 0, nil)
	if err != nil {
		t.Fatalf("StartNewExamSession: %v", err)
	}

	// Only the single-choice is correct: earned 2, no passing score -> immediate.
	submission := examAnswer(
		answer("sc", "3"),
		answer("mc", "9"), // wrong
	)
	got, err := srv.SubmitAnswer(ctx, examId, "user-1", submission, true)
	if err != nil {
		t.Fatalf("SubmitAnswer: %v", err)
	}
	if got.ScoreResult.EarnedScore != 2 {
		t.Errorf("earned = %g, want 2", got.ScoreResult.EarnedScore)
	}
	if got.OverallResult == nil || *got.OverallResult != pkgmodelquestions.OverallResultImmediate {
		t.Errorf("overall = %v, want immediate", got.OverallResult)
	}

	// checkOnly=true must not persist.
	stored, err := srv.GetMyAnswer(ctx, examId, "user-1")
	if err != nil {
		t.Fatalf("GetMyAnswer: %v", err)
	}
	if stored != nil {
		t.Errorf("GetMyAnswer returned %v after checkOnly submit, want nil", stored)
	}
}

// TestGetMyAnswer_NoneSubmitted verifies that GetMyAnswer returns (nil, nil)
// before any non-checkOnly submission has been stored.
func TestGetMyAnswer_NoneSubmitted(t *testing.T) {
	srv := NewOnMemoryExamServer(examreport.NewOnMemoryExamTrackingServer())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	examId, err := srv.StartNewExamSession(ctx, newGradingExam(nil), "user-1", 0, nil)
	if err != nil {
		t.Fatalf("StartNewExamSession: %v", err)
	}

	stored, err := srv.GetMyAnswer(ctx, examId, "user-1")
	if err != nil {
		t.Fatalf("GetMyAnswer before submit: unexpected err %v", err)
	}
	if stored != nil {
		t.Errorf("GetMyAnswer before submit = %v, want nil", stored)
	}
}

// TestGetMyAnswer_Ownership verifies that GetMyAnswer rejects a non-owner caller.
func TestGetMyAnswer_Ownership(t *testing.T) {
	srv := NewOnMemoryExamServer(examreport.NewOnMemoryExamTrackingServer())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	examId, err := srv.StartNewExamSession(ctx, newGradingExam(nil), "user-1", 0, nil)
	if err != nil {
		t.Fatalf("StartNewExamSession: %v", err)
	}

	if _, err := srv.GetMyAnswer(ctx, examId, "user-2"); err != errNotOwner {
		t.Errorf("GetMyAnswer by non-owner = %v, want errNotOwner", err)
	}
}

// TestGetMyAnswer_NotFound verifies that GetMyAnswer on an unknown session
// returns errExamNotFound.
func TestGetMyAnswer_NotFound(t *testing.T) {
	srv := NewOnMemoryExamServer(examreport.NewOnMemoryExamTrackingServer())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	if _, err := srv.GetMyAnswer(ctx, "no-such-session", "user-1"); err != errExamNotFound {
		t.Errorf("GetMyAnswer on unknown session = %v, want errExamNotFound", err)
	}
}

// TestSubmitAnswer_Overwrites verifies that a second non-checkOnly submission
// replaces the previously stored answer.
func TestSubmitAnswer_Overwrites(t *testing.T) {
	srv := NewOnMemoryExamServer(examreport.NewOnMemoryExamTrackingServer())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	defer srv.Shutdown()

	examId, err := srv.StartNewExamSession(ctx, newGradingExam(nil), "user-1", 0, nil)
	if err != nil {
		t.Fatalf("StartNewExamSession: %v", err)
	}

	first := examAnswer(answer("sc", "3"))
	if _, err := srv.SubmitAnswer(ctx, examId, "user-1", first, false); err != nil {
		t.Fatalf("first SubmitAnswer: %v", err)
	}

	second := examAnswer(answer("sc", "9"), answer("mc", "1", "2"))
	if _, err := srv.SubmitAnswer(ctx, examId, "user-1", second, false); err != nil {
		t.Fatalf("second SubmitAnswer: %v", err)
	}

	stored, err := srv.GetMyAnswer(ctx, examId, "user-1")
	if err != nil {
		t.Fatalf("GetMyAnswer: %v", err)
	}
	if stored == nil {
		t.Fatal("GetMyAnswer returned nil after two submits")
	}
	if len(stored.Answers) != 2 { // the second submission had 2 answers
		t.Errorf("stored %d answers, want 2 (latest submission)", len(stored.Answers))
	}
}
