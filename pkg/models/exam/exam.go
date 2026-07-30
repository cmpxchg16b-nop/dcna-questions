package exam

import (
	"context"
	pkgmodelquestions "dcna-questions/pkg/models/question"
	"sync"
)

type ExamId string
type QuestionCursor string

type ExamOptions uint32
const (
	ExamOptionRandomQuestions ExamOptions = 1 << iota
	ExamOptionRandomOptions
	ExamOptionSeekable // the 'Seekable' allows the client to seek to question with given number at will
)

type ExamServer interface {
	StartNewExam(ctx context.Context, userSessionId string, examOptions ExamOptions) (ExamId, error)
	ListExams(ctx context.Context, userSessionId string) []ExamId
	EndExam(ctx context.Context, examId ExamId) error

	// the initial cursor should be nil
	// if has more, `nextCursor` won't be nil
	GetNextQuestion(ctx context.Context, examId ExamId, cursor *QuestionCursor) (question *pkgmodelquestions.Question, nextCursor *QuestionCursor, err error)

	// if the cursor is nil, a brand-new cursor will be created, you should always use the `repositionedCursor` to get the next question, whether it was succeeded or not.
	// if the exam is un-seekable, the operation will fail.
	// the content of cursor is opaque, the client should never assume anything about it.
	SeekCursorTo(ctx context.Context, examId ExamId, cursor *QuestionCursor, newIndex int) (repositionedCursor *QuestionCursor, err error)

	SubmitAnswer(ctx context.Context, examId ExamId, answerXML string, checkOnly bool) (assessmentXML string, err error)
}

// OnMemoryExamServer implements ExamServer
type OnMemoryExamServer struct {
	sessionsStore map[ExamId]*OnMemoryExamSession
	closeDoer sync.Once
	serviceChan chan struct{} // type tbd
	examOptions ExamOptions
}

func NewOnMemoryExamServer(questions pkgmodelquestions.Questions, examOptions ExamOptions) *OnMemoryExamServer {
	// todo
	return nil
}

func (srv *OnMemoryExamServer) Run(ctx context.Context) {}

func (srv *OnMemoryExamServer) Shutdown() {
	// wrap with sync.Once to ensure that repetitive calls of this method be no-op
}

type OnMemoryQuestion struct {
	Question *pkgmodelquestions.Question

	// a nil Permutation is identical to the identical permutation: id: x -> x
	OptionPermutation []int
}

// the singleton OnMemoryExamServer should be the sole ownership holder of every OnMemoryExamSession
type OnMemoryExamSession struct {
	ExamId ExamId
	UserSessionId string
	Questions *pkgmodelquestions.Questions

	// this would be the map of virtual question index to actual question index
	// a nil Permutation is identical to the identical permutation: id: x -> x
	QuestionPermutation []int

	// map[questionId]OnMemoryQuestion, you should always try this first, since it stored the question with shuffuled options order.
	CachedQuestion map[string]OnMemoryQuestion

	// for OnMemoryExamServer/OnMemoryExamSession, cursor id should be uuid
	Cursors map[string]int
}
