package exam

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	pkgmodelquestions "dcna-questions/pkg/models/question"

	"github.com/google/uuid"
)

type ExamId string
type QuestionCursor string

type ExamOptions uint32

const (
	ExamOptionRandomQuestions ExamOptions = 1 << iota
	ExamOptionRandomOptions
	ExamOptionSeekable // the 'Seekable' allows the client to seek to question with given number at will
)

var (
	errExamNotFound  = errors.New("exam not found")
	errNotSeekable   = errors.New("exam is not seekable")
	errInvalidCursor = errors.New("invalid question cursor")
	errOutOfRange    = errors.New("question index out of range")
	errShuttingDown  = errors.New("exam server is shutting down")
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

// OnMemoryExamServer implements ExamServer as a single CSP actor.
//
// All mutable state lives in sessionsStore, and sessionsStore is owned by
// exactly one goroutine: the one running Run. Every public method hands a
// closure to that goroutine through serviceChan; the closure executes with
// exclusive access to the map. No mutexes (other than the closeDoer used to
// make Shutdown idempotent) guard session state.
type OnMemoryExamServer struct {
	questions   pkgmodelquestions.Questions
	examOptions ExamOptions

	// sessionsStore is only ever read or written inside closures run by the
	// actor goroutine.
	sessionsStore map[ExamId]*OnMemoryExamSession

	// serviceChan carries closures for the actor goroutine to run.
	serviceChan chan func()

	// done is closed by Shutdown to release the actor loop and any callers
	// blocked dispatching a command.
	done chan struct{}

	closeDoer sync.Once

	// rng is owned by the actor goroutine and is therefore lock-free. Using a
	// private source (instead of the package-level rand functions) keeps the
	// server free of the math/rand global lock.
	rng *rand.Rand
}

// NewOnMemoryExamServer constructs an in-memory exam server backed by questions.
// examOptions act as a server-wide baseline that is combined (bitwise OR) with
// the per-exam options passed to StartNewExam.
func NewOnMemoryExamServer(questions pkgmodelquestions.Questions, examOptions ExamOptions) *OnMemoryExamServer {
	return &OnMemoryExamServer{
		questions:     questions,
		examOptions:   examOptions,
		sessionsStore: make(map[ExamId]*OnMemoryExamSession),
		serviceChan:   make(chan func()),
		done:          make(chan struct{}),
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Run is the actor loop. Run it in its own goroutine; it returns when ctx is
// canceled or Shutdown is called, after which method calls report errShuttingDown.
func (srv *OnMemoryExamServer) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-srv.done:
			return
		case cmd := <-srv.serviceChan:
			cmd()
		}
	}
}

// Shutdown stops the actor. Idempotent: repeated calls are no-ops via closeDoer.
func (srv *OnMemoryExamServer) Shutdown() {
	srv.closeDoer.Do(func() {
		close(srv.done)
	})
}

// dispatch delivers cmd to the actor goroutine. Because serviceChan is
// unbuffered, a nil return guarantees the actor received cmd and will run it to
// completion, so the caller may safely wait on its response channel.
func (srv *OnMemoryExamServer) dispatch(ctx context.Context, cmd func()) error {
	select {
	case srv.serviceChan <- cmd:
		return nil
	case <-srv.done:
		return errShuttingDown
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (srv *OnMemoryExamServer) StartNewExam(ctx context.Context, userSessionId string, examOptions ExamOptions) (ExamId, error) {
	type result struct {
		examId ExamId
		err    error
	}
	resp := make(chan result, 1)
	cmd := func() {
		opts := srv.examOptions | examOptions
		examId := ExamId(uuid.NewString())
		srv.sessionsStore[examId] = newExamSession(examId, userSessionId, &srv.questions, opts, srv.rng)
		resp <- result{examId: examId}
	}
	if err := srv.dispatch(ctx, cmd); err != nil {
		return "", err
	}
	select {
	case r := <-resp:
		return r.examId, r.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (srv *OnMemoryExamServer) ListExams(ctx context.Context, userSessionId string) []ExamId {
	type result struct{ ids []ExamId }
	resp := make(chan result, 1)
	cmd := func() {
		var ids []ExamId
		for id, s := range srv.sessionsStore {
			if s.UserSessionId == userSessionId {
				ids = append(ids, id)
			}
		}
		resp <- result{ids: ids}
	}
	if err := srv.dispatch(ctx, cmd); err != nil {
		return nil
	}
	select {
	case r := <-resp:
		return r.ids
	case <-ctx.Done():
		return nil
	}
}

func (srv *OnMemoryExamServer) EndExam(ctx context.Context, examId ExamId) error {
	type result struct{ err error }
	resp := make(chan result, 1)
	cmd := func() {
		if _, ok := srv.sessionsStore[examId]; !ok {
			resp <- result{err: errExamNotFound}
			return
		}
		delete(srv.sessionsStore, examId)
		resp <- result{}
	}
	if err := srv.dispatch(ctx, cmd); err != nil {
		return err
	}
	select {
	case r := <-resp:
		return r.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (srv *OnMemoryExamServer) GetNextQuestion(ctx context.Context, examId ExamId, cursor *QuestionCursor) (question *pkgmodelquestions.Question, nextCursor *QuestionCursor, err error) {
	type result struct {
		question   *pkgmodelquestions.Question
		nextCursor *QuestionCursor
		err        error
	}
	resp := make(chan result, 1)
	cmd := func() {
		sess, ok := srv.sessionsStore[examId]
		if !ok {
			resp <- result{err: errExamNotFound}
			return
		}
		idx := 0
		if cursor != nil {
			if idx, ok = sess.Cursors[string(*cursor)]; !ok {
				resp <- result{err: errInvalidCursor}
				return
			}
		}
		perm := sess.QuestionPermutation
		if idx < 0 || idx >= len(perm) {
			// No more questions: nil question, nil cursor, no error.
			resp <- result{}
			return
		}
		question = sess.cachedQuestion(perm[idx], srv.rng)
		if idx+1 < len(perm) {
			c := QuestionCursor(uuid.NewString())
			sess.Cursors[string(c)] = idx + 1
			nextCursor = &c
		}
		resp <- result{question: question, nextCursor: nextCursor}
	}
	if err := srv.dispatch(ctx, cmd); err != nil {
		return nil, nil, err
	}
	select {
	case r := <-resp:
		return r.question, r.nextCursor, r.err
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

func (srv *OnMemoryExamServer) SeekCursorTo(ctx context.Context, examId ExamId, cursor *QuestionCursor, newIndex int) (*QuestionCursor, error) {
	type result struct {
		cursor *QuestionCursor
		err    error
	}
	resp := make(chan result, 1)
	cmd := func() {
		sess, ok := srv.sessionsStore[examId]
		if !ok {
			resp <- result{err: errExamNotFound}
			return
		}
		if sess.Options&ExamOptionSeekable == 0 {
			resp <- result{err: errNotSeekable}
			return
		}
		if newIndex < 0 || newIndex >= len(sess.QuestionPermutation) {
			resp <- result{err: errOutOfRange}
			return
		}
		// Reuse the incoming cursor id when given, otherwise mint a new one.
		id := uuid.NewString()
		if cursor != nil {
			id = string(*cursor)
		}
		sess.Cursors[id] = newIndex
		c := QuestionCursor(id)
		resp <- result{cursor: &c}
	}
	if err := srv.dispatch(ctx, cmd); err != nil {
		return nil, err
	}
	select {
	case r := <-resp:
		return r.cursor, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SubmitAnswer is not yet implemented.
func (srv *OnMemoryExamServer) SubmitAnswer(ctx context.Context, examId ExamId, answerXML string, checkOnly bool) (assessmentXML string, err error) {
	return "", nil
}

type OnMemoryQuestion struct {
	Question *pkgmodelquestions.Question

	// a nil Permutation is identical to the identical permutation: id: x -> x
	OptionPermutation []int
}

// the singleton OnMemoryExamServer should be the sole ownership holder of every OnMemoryExamSession
type OnMemoryExamSession struct {
	ExamId        ExamId
	UserSessionId string
	Questions     *pkgmodelquestions.Questions

	// Options captured for this exam; drives seekable checks and option
	// shuffling when a question is first materialized.
	Options ExamOptions

	// this would be the map of virtual question index to actual question index
	// a nil Permutation is identical to the identical permutation: id: x -> x
	QuestionPermutation []int

	// map[questionId]OnMemoryQuestion, you should always try this first, since it stored the question with shuffuled options order.
	CachedQuestion map[string]OnMemoryQuestion

	// for OnMemoryExamServer/OnMemoryExamSession, cursor id should be uuid
	Cursors map[string]int
}

// cachedQuestion returns the question at actualIdx, building and caching a copy
// with shuffled options on first access. rng is provided by the actor goroutine
// and is therefore used single-threaded.
func (sess *OnMemoryExamSession) cachedQuestion(actualIdx int, rng *rand.Rand) *pkgmodelquestions.Question {
	orig := &(*sess.Questions)[actualIdx]
	if cached, ok := sess.CachedQuestion[orig.Id]; ok {
		return cached.Question
	}
	omq := buildOnMemoryQuestion(orig, sess.Options, rng)
	sess.CachedQuestion[orig.Id] = omq
	return omq.Question
}

// newExamSession allocates a session and computes its question permutation up
// front (the order in which questions are presented). Option permutations are
// derived lazily, per question, as it is first requested.
func newExamSession(examId ExamId, userSessionId string, questions *pkgmodelquestions.Questions, opts ExamOptions, rng *rand.Rand) *OnMemoryExamSession {
	n := len(*questions)
	var qPerm []int
	if opts&ExamOptionRandomQuestions != 0 {
		qPerm = rng.Perm(n)
	} else {
		qPerm = identityPermutation(n)
	}
	return &OnMemoryExamSession{
		ExamId:              examId,
		UserSessionId:       userSessionId,
		Questions:           questions,
		Options:             opts,
		QuestionPermutation: qPerm,
		CachedQuestion:      make(map[string]OnMemoryQuestion),
		Cursors:             make(map[string]int),
	}
}

// buildOnMemoryQuestion returns a shallow copy of orig whose Options are
// reordered according to the (random or identity) option permutation. The
// original question bank is never mutated; option Ids are preserved so
// CorrectAnswer (which references options by value) stays valid.
func buildOnMemoryQuestion(orig *pkgmodelquestions.Question, opts ExamOptions, rng *rand.Rand) OnMemoryQuestion {
	qCopy := *orig
	m := len(orig.Options)
	if m == 0 {
		return OnMemoryQuestion{Question: &qCopy, OptionPermutation: identityPermutation(0)}
	}
	var optPerm []int
	if opts&ExamOptionRandomOptions != 0 {
		optPerm = rng.Perm(m)
	} else {
		optPerm = identityPermutation(m)
	}
	reordered := make(pkgmodelquestions.Options, m)
	for i, p := range optPerm {
		reordered[i] = orig.Options[p]
	}
	qCopy.Options = reordered
	return OnMemoryQuestion{Question: &qCopy, OptionPermutation: optPerm}
}

// identityPermutation returns [0, 1, ..., n-1], the identical permutation,
// generated programmatically.
func identityPermutation(n int) []int {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	return p
}
