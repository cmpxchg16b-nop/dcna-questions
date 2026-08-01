package examserver

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	pkgmodelquestions "dcna-questions/pkg/models/question"

	"github.com/google/uuid"
)

type ExamSessionId string
type QuestionCursor string

type ExamOptions uint32

const (
	ExamOptionRandomQuestions    ExamOptions = 1 << iota // randomized questions ordering within a collection
	ExamOptionRandomOptions                              // randomized options ordering
	ExamOptionSeekable                                   // the 'Seekable' allows the client to seek to question with given number at will
	ExamOptionRandomQuestionColl                         // Randomized question collection picking
)

var (
	errExamNotFound  = errors.New("exam not found")
	errNotOwner      = errors.New("exam session does not belong to the caller")
	errNotSeekable   = errors.New("exam is not seekable")
	errInvalidCursor = errors.New("invalid question cursor")
	errOutOfRange    = errors.New("question index out of range")
	errShuttingDown  = errors.New("exam server is shutting down")
	errEmptyExam     = errors.New("exam has no questions")
)

type ExamSessionExcerpt struct {
	// This Id is the id of exam session, by which the exam server use to keep track of exam sessions
	Id ExamSessionId

	ExamExcerpt pkgmodelquestions.ExamDocumentExcerpt
	// millisecond-resolution unix timestamp
	StartedAt uint64
}

type ExamServer interface {
	// Start a new exam session
	StartNewExamSession(ctx context.Context, exam *pkgmodelquestions.Exam, userId string, examOptions ExamOptions) (ExamSessionId, error)

	// List started exam sessions of a given user
	ListExamSessions(ctx context.Context, userId string) []ExamSessionExcerpt

	// Terminate the specified exam session
	EndExamSession(ctx context.Context, examId ExamSessionId, userId string) error

	// the initial cursor should be nil
	// if has more, `nextCursor` won't be nil
	GetNextQuestion(ctx context.Context, examId ExamSessionId, userId string, cursor *QuestionCursor) (question *pkgmodelquestions.Question, nextCursor *QuestionCursor, err error)

	// if the cursor is nil, a brand-new cursor will be created, you should always use the `repositionedCursor` to get the next question, whether it was succeeded or not.
	// if the exam is un-seekable, the operation will fail.
	// the content of cursor is opaque, the client should never assume anything about it.
	SeekCursorTo(ctx context.Context, examId ExamSessionId, userId string, cursor *QuestionCursor, newVirtualIndex int) (repositionedCursor *QuestionCursor, err error)

	SubmitAnswer(ctx context.Context, examId ExamSessionId, userId string, answer *pkgmodelquestions.ExamAnswer, checkOnly bool) (*pkgmodelquestions.Assessment, error)
}

// OnMemoryExamServer implements ExamServer as a single CSP actor.
//
// All mutable state lives in sessionsStore, and sessionsStore is owned by
// exactly one goroutine: the one running Run. Every public method hands a
// closure to that goroutine through serviceChan; the closure executes with
// exclusive access to the map. No mutexes (other than the closeDoer used to
// make Shutdown idempotent) guard session state.
//
// The server holds no question bank and no RNG: both live per session (see
// OnMemoryExamSession), supplied through StartNewExamSession. Each session's RNG is
// only ever touched inside closures run by the actor goroutine, so it remains
// lock-free.
type OnMemoryExamServer struct {
	// sessionsStore is only ever read or written inside closures run by the
	// actor goroutine.
	sessionsStore map[ExamSessionId]*OnMemoryExamSession

	// userSessions is the reverse index of sessionsStore: it maps a user id
	// to the set of exam ids that user has started, so that ListExamSessions
	// is O(sessions-for-user) instead of scanning every session. Like
	// sessionsStore, it is only ever touched inside closures run by the actor
	// goroutine, so it stays in lock-step with sessionsStore without any
	// locking of its own.
	userSessions map[string]map[ExamSessionId]struct{}

	// serviceChan carries closures for the actor goroutine to run.
	serviceChan chan func()

	// done is closed by Shutdown to release the actor loop and any callers
	// blocked dispatching a command.
	done chan struct{}

	closeDoer sync.Once
}

// NewOnMemoryExamServer constructs an in-memory exam server. The exam options,
// question bank, and RNG are all supplied per exam via StartNewExamSession, not held by
// the server.
func NewOnMemoryExamServer() *OnMemoryExamServer {
	return &OnMemoryExamServer{
		sessionsStore: make(map[ExamSessionId]*OnMemoryExamSession),
		userSessions:  make(map[string]map[ExamSessionId]struct{}),
		serviceChan:   make(chan func()),
		done:          make(chan struct{}),
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

func (srv *OnMemoryExamServer) StartNewExamSession(ctx context.Context, exam *pkgmodelquestions.Exam, userId string, examOptions ExamOptions) (ExamSessionId, error) {
	type result struct {
		examId ExamSessionId
		err    error
	}
	resp := make(chan result, 1)
	cmd := func() {
		if exam == nil || len(exam.QuestionSet.QuestionCollections) == 0 {
			resp <- result{err: errEmptyExam}
			return
		}
		opts := examOptions
		examId := ExamSessionId(uuid.NewString())
		srv.sessionsStore[examId] = newExamSession(examId, userId, exam, opts)
		sessions, ok := srv.userSessions[userId]
		if !ok {
			sessions = make(map[ExamSessionId]struct{})
			srv.userSessions[userId] = sessions
		}
		sessions[examId] = struct{}{}
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

func (srv *OnMemoryExamServer) ListExamSessions(ctx context.Context, userId string) []ExamSessionExcerpt {
	type result struct{ excerpts []ExamSessionExcerpt }
	resp := make(chan result, 1)
	cmd := func() {
		sessions := srv.userSessions[userId]
		excerpts := make([]ExamSessionExcerpt, 0, len(sessions))
		for id := range sessions {
			sess := srv.sessionsStore[id]
			excerpts = append(excerpts, ExamSessionExcerpt{
				Id:          id,
				ExamExcerpt: pkgmodelquestions.ExamExcerptFrom(sess.Exam),
				StartedAt:   sess.StartedAt,
			})
		}
		resp <- result{excerpts: excerpts}
	}
	if err := srv.dispatch(ctx, cmd); err != nil {
		return nil
	}
	select {
	case r := <-resp:
		return r.excerpts
	case <-ctx.Done():
		return nil
	}
}

func (srv *OnMemoryExamServer) EndExamSession(ctx context.Context, examId ExamSessionId, userId string) error {
	type result struct{ err error }
	resp := make(chan result, 1)
	cmd := func() {
		sess, ok := srv.sessionsStore[examId]
		if !ok {
			resp <- result{err: errExamNotFound}
			return
		}
		if sess.UserId != userId {
			resp <- result{err: errNotOwner}
			return
		}
		delete(srv.sessionsStore, examId)
		if sessions, ok := srv.userSessions[sess.UserId]; ok {
			delete(sessions, examId)
			if len(sessions) == 0 {
				delete(srv.userSessions, sess.UserId)
			}
		}
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

func (srv *OnMemoryExamServer) GetNextQuestion(ctx context.Context, examId ExamSessionId, userId string, cursor *QuestionCursor) (question *pkgmodelquestions.Question, nextCursor *QuestionCursor, err error) {
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
		if sess.UserId != userId {
			resp <- result{err: errNotOwner}
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
		question = sess.cachedQuestion(perm[idx])
		if idx+1 < len(perm) {
			// The cursor's meaning is unchanged ("next question to read"), so
			// advance it in place instead of minting a new token. On the very
			// first call the incoming cursor is nil, so one is created.
			c := cursor
			if c == nil {
				fresh := QuestionCursor(uuid.NewString())
				c = &fresh
			}
			sess.Cursors[string(*c)] = idx + 1
			nextCursor = c
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

func (srv *OnMemoryExamServer) SeekCursorTo(ctx context.Context, examId ExamSessionId, userId string, cursor *QuestionCursor, newIndex int) (*QuestionCursor, error) {
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
		if sess.UserId != userId {
			resp <- result{err: errNotOwner}
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
		// Seeking repositions traversal: mint a fresh cursor and invalidate
		// the old one so it can no longer be used.
		newId := uuid.NewString()
		sess.Cursors[newId] = newIndex
		if cursor != nil {
			delete(sess.Cursors, string(*cursor))
		}
		c := QuestionCursor(newId)
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

// SubmitAnswer is not yet fully implemented: it resolves and authorizes the
// session, but does not yet grade the submitted answer. A successful call
// returns a nil assessment until grading is added.
func (srv *OnMemoryExamServer) SubmitAnswer(ctx context.Context, examId ExamSessionId, userId string, answer *pkgmodelquestions.ExamAnswer, checkOnly bool) (*pkgmodelquestions.Assessment, error) {
	type result struct {
		assessment *pkgmodelquestions.Assessment
		err        error
	}
	resp := make(chan result, 1)
	cmd := func() {
		sess, ok := srv.sessionsStore[examId]
		if !ok {
			resp <- result{err: errExamNotFound}
			return
		}
		if sess.UserId != userId {
			resp <- result{err: errNotOwner}
			return
		}
		// Grading is not yet implemented.
		resp <- result{}
	}
	if err := srv.dispatch(ctx, cmd); err != nil {
		return nil, err
	}
	select {
	case r := <-resp:
		return r.assessment, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type OnMemoryQuestion struct {
	Question *pkgmodelquestions.Question

	// a nil Permutation is identical to the identical permutation: id: x -> x
	OptionPermutation []int
}

// the singleton OnMemoryExamServer should be the sole ownership holder of every OnMemoryExamSession
type OnMemoryExamSession struct {
	ExamId    ExamSessionId
	UserId    string
	Exam      *pkgmodelquestions.Exam
	Questions *pkgmodelquestions.QuestionCollection

	// StartedAt is the millisecond-resolution unix timestamp captured when the
	// session was created; it is surfaced unchanged through ListExamSessions.
	StartedAt uint64

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

	// rng is the per-session random source, used to shuffle the question order
	// and each question's options. It is owned by the actor goroutine (touched
	// only inside dispatch closures) and is therefore lock-free.
	rng *rand.Rand

	// ExamAnswer holds the user's latest submitted answer (the parsed <examanswer>
	// element; see exam1.xml for its shape). When SubmitAnswer is called with
	// checkOnly set to true, this field is not updated.
	ExamAnswer *pkgmodelquestions.ExamAnswer
}

// cachedQuestion returns the question at actualIdx, building and caching a copy
// with shuffled options on first access. The session's rng is used, which is
// owned by the actor goroutine and therefore single-threaded.
func (sess *OnMemoryExamSession) cachedQuestion(actualIdx int) *pkgmodelquestions.Question {
	orig := &sess.Questions.Questions[actualIdx]
	if cached, ok := sess.CachedQuestion[orig.Id]; ok {
		return cached.Question
	}
	omq := buildOnMemoryQuestion(orig, sess.Options, sess.rng)
	sess.CachedQuestion[orig.Id] = omq
	return omq.Question
}

// newExamSession allocates a session backed by exam's question set, selecting
// the question collection to present and computing its permutation up front
// (the order in which questions are presented). Option permutations are derived
// lazily, per question, as it is first requested.
func newExamSession(examId ExamSessionId, userId string, exam *pkgmodelquestions.Exam, opts ExamOptions) *OnMemoryExamSession {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	qc := selectQuestionCollection(exam.QuestionSet, opts, rng)
	n := len(qc.Questions)
	var qPerm []int
	if opts&ExamOptionRandomQuestions != 0 {
		qPerm = rng.Perm(n)
	} else {
		qPerm = identityPermutation(n)
	}
	return &OnMemoryExamSession{
		ExamId:              examId,
		UserId:              userId,
		Exam:                exam,
		StartedAt:           uint64(time.Now().UnixMilli()),
		Questions:           &qc,
		Options:             opts,
		QuestionPermutation: qPerm,
		CachedQuestion:      make(map[string]OnMemoryQuestion),
		Cursors:             make(map[string]int),
		rng:                 rng,
	}
}

// selectQuestionCollection resolves which QuestionCollection from the exam's
// question set is presented to a candidate.
//
// With ExamOptionRandomQuestionColl set, one collection is picked at random
// (the point of a multi-collection set: vary the exam by drawing a different
// subset). Otherwise every collection's questions are flattened into a single
// combined collection, so the candidate sees all questions.
func selectQuestionCollection(qs pkgmodelquestions.QuestionSet, opts ExamOptions, rng *rand.Rand) pkgmodelquestions.QuestionCollection {
	cols := qs.QuestionCollections
	if len(cols) == 0 {
		return pkgmodelquestions.QuestionCollection{}
	}
	if opts&ExamOptionRandomQuestionColl != 0 {
		return cols[rng.Intn(len(cols))]
	}
	// Flatten all collections into one so the candidate sees every question.
	var total int
	for _, c := range cols {
		total += len(c.Questions)
	}
	flat := make([]pkgmodelquestions.Question, 0, total)
	for _, c := range cols {
		flat = append(flat, c.Questions...)
	}
	return pkgmodelquestions.QuestionCollection{Questions: flat}
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
