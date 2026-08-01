package examsessions_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dcna-questions/pkg/api/examsessions"
	"dcna-questions/pkg/models/examserver"
	"dcna-questions/pkg/models/question"
	"dcna-questions/pkg/session"
)

// fakeExamLoader is an ExamLoader that serves a fixed set of exams keyed by URL.
type fakeExamLoader struct {
	exams map[string]*question.Exam
}

func (l *fakeExamLoader) LoadFrom(url string) (*question.Exam, error) {
	if e, ok := l.exams[url]; ok {
		return e, nil
	}
	return nil, errors.New("exam not found")
}

// fakeExamServer is an ExamServer that records calls and returns canned results.
// Only the methods the handler exercises have meaningful state; the rest are
// stubs present solely to satisfy the ExamServer interface.
type fakeExamServer struct {
	startErr    error
	startID     examserver.ExamSessionId
	startedExam *question.Exam
	startedUser string
	startedOpts examserver.ExamOptions

	listResult []examserver.ExamSessionExcerpt
	listCalls  []string

	endErr error
	ended  []examserver.ExamSessionId

	// GetNextQuestion
	gnqQuestion *question.Question
	gnqNext     *examserver.QuestionCursor
	gnqErr      error
	gnqExamID   string
	gnqCursorIn *examserver.QuestionCursor

	// SeekCursorTo
	seekResult   *examserver.QuestionCursor
	seekErr      error
	seekExamID   string
	seekCursorIn *examserver.QuestionCursor
	seekIndex    int
}

func (s *fakeExamServer) StartNewExamSession(_ context.Context, exam *question.Exam, userSessionId string, opts examserver.ExamOptions) (examserver.ExamSessionId, error) {
	s.startedExam = exam
	s.startedUser = userSessionId
	s.startedOpts = opts
	if s.startErr != nil {
		return "", s.startErr
	}
	if s.startID != "" {
		return s.startID, nil
	}
	return "default-session", nil
}

func (s *fakeExamServer) ListExamSessions(_ context.Context, userSessionId string) []examserver.ExamSessionExcerpt {
	s.listCalls = append(s.listCalls, userSessionId)
	return s.listResult
}

func (s *fakeExamServer) GetExamSessionById(_ context.Context, examId examserver.ExamSessionId, _ string) (examserver.ExamSessionExcerpt, error) {
	return examserver.ExamSessionExcerpt{}, nil
}

func (s *fakeExamServer) EndExamSession(_ context.Context, examId examserver.ExamSessionId, userId string) error {
	s.ended = append(s.ended, examId)
	return s.endErr
}

func (s *fakeExamServer) GetNextQuestion(_ context.Context, examId examserver.ExamSessionId, userId string, cursor *examserver.QuestionCursor) (*question.Question, *examserver.QuestionCursor, error) {
	s.gnqExamID = string(examId)
	s.gnqCursorIn = cursor
	if s.gnqErr != nil {
		return nil, nil, s.gnqErr
	}
	return s.gnqQuestion, s.gnqNext, nil
}

func (s *fakeExamServer) SeekCursorTo(_ context.Context, examId examserver.ExamSessionId, userId string, cursor *examserver.QuestionCursor, index int) (*examserver.QuestionCursor, error) {
	s.seekExamID = string(examId)
	s.seekCursorIn = cursor
	s.seekIndex = index
	if s.seekErr != nil {
		return nil, s.seekErr
	}
	return s.seekResult, nil
}

func (s *fakeExamServer) SubmitAnswer(context.Context, examserver.ExamSessionId, string, *question.ExamAnswer, bool) (*question.Assessment, error) {
	return nil, nil
}

// newTestExam builds a minimal valid exam with id and one single-choice question.
func newTestExam(id string) *question.Exam {
	return &question.Exam{
		Id: id,
		QuestionSet: question.QuestionSet{
			QuestionCollections: []question.QuestionCollection{
				{Questions: []question.Question{
					{Id: id + "-q1", Type: question.QuestionTypeSingleChoice, Score: 1},
				}},
			},
		},
	}
}

// testQuestion returns a minimal question for assertion targets.
func testQuestion(id string) *question.Question {
	return &question.Question{Id: id, Type: question.QuestionTypeSingleChoice}
}

// ptrCursor boxes a cursor string so it can populate a *examserver.QuestionCursor
// field in a struct literal.
func ptrCursor(s string) *examserver.QuestionCursor {
	c := examserver.QuestionCursor(s)
	return &c
}

// decodeJSON unmarshals body into v, failing the test on error.
func decodeJSON(t *testing.T, body string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), v); err != nil {
		t.Fatalf("decode body %q: %v", body, err)
	}
}

// newRepoWith wraps the exams in an ExamRepository backed by a fake loader, so
// each exam is retrievable by its Id via GetExamDocumentById.
func newRepoWith(exams ...*question.Exam) *question.ExamRepository {
	byURL := make(map[string]*question.Exam, len(exams))
	urls := make([]string, 0, len(exams))
	for _, e := range exams {
		url := "memory://" + e.Id
		byURL[url] = e
		urls = append(urls, url)
	}
	return question.NewExamRepository([]question.ExamSource{
		{Loader: &fakeExamLoader{exams: byURL}, URLs: urls},
	})
}

// testEnv wires a handler behind a ServeMux that mirrors the documented mount.
type testEnv struct {
	sm     *session.OnMemorySessionManager
	server *fakeExamServer
	sess   *session.Session
	mux    *http.ServeMux
}

func newTestEnv(t *testing.T, repo *question.ExamRepository, server *fakeExamServer) *testEnv {
	t.Helper()
	sm := session.NewOnMemorySessionManager()
	_, sess := sm.CreateSession()
	h := examsessions.NewExamSessionHandler(sm, server, repo)
	mux := http.NewServeMux()
	mux.Handle("/api/examsessions", h)
	mux.Handle("/api/examsessions/", h)
	return &testEnv{sm: sm, server: server, sess: sess, mux: mux}
}

func (e *testEnv) serve(t *testing.T, method, target, body string, withSession bool) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	if withSession {
		r = r.WithContext(e.sm.WithSession(r.Context(), e.sess))
	}
	rr := httptest.NewRecorder()
	e.mux.ServeHTTP(rr, r)
	return rr
}

func TestExamSessionHandler(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		target     string
		body       string
		noSession  bool
		exam       *question.Exam
		server     *fakeExamServer
		wantStatus int
		wantAllow  string
		checkBody  func(t *testing.T, body string)
		checkCalls func(t *testing.T, s *fakeExamServer, sessID string)
	}{
		{
			name:       "create success",
			method:     http.MethodPost,
			target:     "/api/examsessions",
			body:       `{"exam_id":"exam-1"}`,
			exam:       newTestExam("exam-1"),
			server:     &fakeExamServer{startID: "sess-1"},
			wantStatus: http.StatusCreated,
			checkBody: func(t *testing.T, body string) {
				var got struct {
					ExamSessionID string `json:"exam_session_id"`
				}
				if err := json.Unmarshal([]byte(body), &got); err != nil {
					t.Fatalf("decode body %q: %v", body, err)
				}
				if got.ExamSessionID != "sess-1" {
					t.Errorf("exam_session_id = %q, want sess-1", got.ExamSessionID)
				}
			},
			checkCalls: func(t *testing.T, s *fakeExamServer, sessID string) {
				if s.startedExam == nil || s.startedExam.Id != "exam-1" {
					t.Errorf("started exam = %+v, want id exam-1", s.startedExam)
				}
				if s.startedUser != sessID {
					t.Errorf("started user = %q, want %q", s.startedUser, sessID)
				}
				if s.startedOpts != 0 {
					t.Errorf("started opts = %d, want 0 when options absent", s.startedOpts)
				}
			},
		},
		{
			name:       "create success with options",
			method:     http.MethodPost,
			target:     "/api/examsessions",
			body:       `{"exam_id":"exam-1","options":3}`,
			exam:       newTestExam("exam-1"),
			server:     &fakeExamServer{startID: "sess-1"},
			wantStatus: http.StatusCreated,
			checkCalls: func(t *testing.T, s *fakeExamServer, sessID string) {
				want := examserver.ExamOptionRandomQuestions | examserver.ExamOptionRandomOptions
				if s.startedOpts != want {
					t.Errorf("started opts = %d, want %d", s.startedOpts, want)
				}
			},
		},
		{
			name:       "create missing exam_id",
			method:     http.MethodPost,
			target:     "/api/examsessions",
			body:       `{}`,
			server:     &fakeExamServer{},
			wantStatus: http.StatusBadRequest,
			checkCalls: func(t *testing.T, s *fakeExamServer, sessID string) {
				if s.startedExam != nil {
					t.Errorf("server should not be called, got startedExam id %q", s.startedExam.Id)
				}
			},
		},
		{
			name:       "create invalid JSON",
			method:     http.MethodPost,
			target:     "/api/examsessions",
			body:       `not-json`,
			server:     &fakeExamServer{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "create empty body",
			method:     http.MethodPost,
			target:     "/api/examsessions",
			body:       ``,
			server:     &fakeExamServer{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "create exam not found",
			method:     http.MethodPost,
			target:     "/api/examsessions",
			body:       `{"exam_id":"missing"}`,
			exam:       newTestExam("exam-1"),
			server:     &fakeExamServer{},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "create exam has no questions",
			method:     http.MethodPost,
			target:     "/api/examsessions",
			body:       `{"exam_id":"empty"}`,
			exam:       &question.Exam{Id: "empty"},
			server:     &fakeExamServer{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "create server error",
			method:     http.MethodPost,
			target:     "/api/examsessions",
			body:       `{"exam_id":"exam-1"}`,
			exam:       newTestExam("exam-1"),
			server:     &fakeExamServer{startErr: errors.New("shutdown")},
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:   "list success",
			method: http.MethodGet,
			target: "/api/examsessions",
			server: &fakeExamServer{listResult: []examserver.ExamSessionExcerpt{
				{Id: "a", ExamExcerpt: question.ExamDocumentExcerpt{Id: "exam-a"}, StartedAt: 1000},
				{Id: "b", ExamExcerpt: question.ExamDocumentExcerpt{Id: "exam-b"}, StartedAt: 2000},
			}},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				var got struct {
					ExamSessions []struct {
						ExamSessionID string                       `json:"exam_session_id"`
						ExamExcerpt   question.ExamDocumentExcerpt `json:"exam_excerpt"`
						StartedAt     uint64                       `json:"started_at"`
					} `json:"exam_sessions"`
				}
				if err := json.Unmarshal([]byte(body), &got); err != nil {
					t.Fatalf("decode body %q: %v", body, err)
				}
				want := []struct {
					examSessionID string
					examID        string
					startedAt     uint64
				}{{"a", "exam-a", 1000}, {"b", "exam-b", 2000}}
				if len(got.ExamSessions) != len(want) {
					t.Fatalf("exam_sessions = %+v, want %d entries", got.ExamSessions, len(want))
				}
				for i, w := range want {
					if got.ExamSessions[i].ExamSessionID != w.examSessionID {
						t.Errorf("exam_sessions[%d].exam_session_id = %q, want %q", i, got.ExamSessions[i].ExamSessionID, w.examSessionID)
					}
					if got.ExamSessions[i].ExamExcerpt.Id != w.examID {
						t.Errorf("exam_sessions[%d].exam_excerpt.Id = %q, want %q", i, got.ExamSessions[i].ExamExcerpt.Id, w.examID)
					}
					if got.ExamSessions[i].StartedAt != w.startedAt {
						t.Errorf("exam_sessions[%d].started_at = %d, want %d", i, got.ExamSessions[i].StartedAt, w.startedAt)
					}
				}
			},
			checkCalls: func(t *testing.T, s *fakeExamServer, sessID string) {
				if len(s.listCalls) != 1 || s.listCalls[0] != sessID {
					t.Errorf("list calls = %v, want [%s]", s.listCalls, sessID)
				}
			},
		},
		{
			name:       "list empty",
			method:     http.MethodGet,
			target:     "/api/examsessions",
			server:     &fakeExamServer{},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				var got struct {
					ExamSessions []json.RawMessage `json:"exam_sessions"`
				}
				if err := json.Unmarshal([]byte(body), &got); err != nil {
					t.Fatalf("decode body %q: %v", body, err)
				}
				if len(got.ExamSessions) != 0 {
					t.Errorf("exam_sessions = %v, want empty", got.ExamSessions)
				}
			},
		},
		{
			name:       "delete success",
			method:     http.MethodDelete,
			target:     "/api/examsessions/sess-x",
			server:     &fakeExamServer{},
			wantStatus: http.StatusNoContent,
			checkCalls: func(t *testing.T, s *fakeExamServer, sessID string) {
				if len(s.ended) != 1 || s.ended[0] != "sess-x" {
					t.Errorf("ended = %v, want [sess-x]", s.ended)
				}
			},
		},
		{
			name:       "delete not found",
			method:     http.MethodDelete,
			target:     "/api/examsessions/sess-x",
			server:     &fakeExamServer{endErr: errors.New("not found")},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "get next question success",
			method:     http.MethodGet,
			target:     "/api/examsessions/sess-1/questions?cursor_id=cur-1",
			server:     &fakeExamServer{gnqQuestion: testQuestion("q-1"), gnqNext: ptrCursor("cur-2")},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				var got struct {
					CursorID *string            `json:"cursor_id"`
					Question *question.Question `json:"question"`
				}
				decodeJSON(t, body, &got)
				if got.CursorID == nil || *got.CursorID != "cur-2" {
					t.Errorf("cursor_id = %v, want cur-2", got.CursorID)
				}
				if got.Question == nil || got.Question.Id != "q-1" {
					t.Errorf("question = %+v, want id q-1", got.Question)
				}
			},
			checkCalls: func(t *testing.T, s *fakeExamServer, sessID string) {
				if s.gnqExamID != "sess-1" {
					t.Errorf("GetNextQuestion exam id = %q, want sess-1", s.gnqExamID)
				}
				if s.gnqCursorIn == nil || string(*s.gnqCursorIn) != "cur-1" {
					t.Errorf("GetNextQuestion cursor = %v, want cur-1", s.gnqCursorIn)
				}
			},
		},
		{
			name:       "get next question no more",
			method:     http.MethodGet,
			target:     "/api/examsessions/sess-1/questions",
			server:     &fakeExamServer{},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				var got struct {
					CursorID *string            `json:"cursor_id"`
					Question *question.Question `json:"question"`
				}
				decodeJSON(t, body, &got)
				if got.CursorID != nil {
					t.Errorf("cursor_id = %v, want null", got.CursorID)
				}
				if got.Question != nil {
					t.Errorf("question = %+v, want null", got.Question)
				}
			},
			checkCalls: func(t *testing.T, s *fakeExamServer, sessID string) {
				if s.gnqCursorIn != nil {
					t.Errorf("initial cursor = %v, want nil", s.gnqCursorIn)
				}
			},
		},
		{
			name:       "get next question error",
			method:     http.MethodGet,
			target:     "/api/examsessions/sess-1/questions",
			server:     &fakeExamServer{gnqErr: errors.New("exam not found")},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "seek cursor success",
			method:     http.MethodPut,
			target:     "/api/examsessions/sess-1/cursors?cursor_id=cur-1&index=3",
			server:     &fakeExamServer{seekResult: ptrCursor("cur-2")},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				var got struct {
					CursorID string `json:"cursor_id"`
				}
				decodeJSON(t, body, &got)
				if got.CursorID != "cur-2" {
					t.Errorf("cursor_id = %q, want cur-2", got.CursorID)
				}
			},
			checkCalls: func(t *testing.T, s *fakeExamServer, sessID string) {
				if s.seekExamID != "sess-1" {
					t.Errorf("SeekCursorTo exam id = %q, want sess-1", s.seekExamID)
				}
				if s.seekIndex != 3 {
					t.Errorf("SeekCursorTo index = %d, want 3", s.seekIndex)
				}
				if s.seekCursorIn == nil || string(*s.seekCursorIn) != "cur-1" {
					t.Errorf("SeekCursorTo cursor = %v, want cur-1", s.seekCursorIn)
				}
			},
		},
		{
			name:       "seek cursor missing index",
			method:     http.MethodPut,
			target:     "/api/examsessions/sess-1/cursors?cursor_id=cur-1",
			server:     &fakeExamServer{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "seek cursor invalid index",
			method:     http.MethodPut,
			target:     "/api/examsessions/sess-1/cursors?index=-1",
			server:     &fakeExamServer{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "seek cursor error",
			method:     http.MethodPut,
			target:     "/api/examsessions/sess-1/cursors?index=0",
			server:     &fakeExamServer{seekErr: errors.New("exam is not seekable")},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "method not allowed on questions",
			method:     http.MethodPost,
			target:     "/api/examsessions/sess-1/questions",
			server:     &fakeExamServer{},
			wantStatus: http.StatusMethodNotAllowed,
			wantAllow:  "GET",
		},
		{
			name:       "method not allowed on cursors",
			method:     http.MethodGet,
			target:     "/api/examsessions/sess-1/cursors",
			server:     &fakeExamServer{},
			wantStatus: http.StatusMethodNotAllowed,
			wantAllow:  "PUT",
		},
		{
			name:       "unknown sub-path not found",
			method:     http.MethodGet,
			target:     "/api/examsessions/sess-1/unknown",
			server:     &fakeExamServer{},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "method not allowed on collection",
			method:     http.MethodPut,
			target:     "/api/examsessions",
			server:     &fakeExamServer{},
			wantStatus: http.StatusMethodNotAllowed,
			wantAllow:  "GET, POST",
		},
		{
			name:       "method not allowed on item",
			method:     http.MethodGet,
			target:     "/api/examsessions/sess-x",
			server:     &fakeExamServer{},
			wantStatus: http.StatusMethodNotAllowed,
			wantAllow:  "DELETE",
		},
		{
			name:       "no session in context",
			method:     http.MethodGet,
			target:     "/api/examsessions",
			noSession:  true,
			server:     &fakeExamServer{},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepoWith()
			if tc.exam != nil {
				repo = newRepoWith(tc.exam)
			}
			env := newTestEnv(t, repo, tc.server)
			rr := env.serve(t, tc.method, tc.target, tc.body, !tc.noSession)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", rr.Code, tc.wantStatus, rr.Body.String())
			}
			if tc.wantAllow != "" {
				if got := rr.Header().Get("Allow"); got != tc.wantAllow {
					t.Errorf("Allow = %q, want %q", got, tc.wantAllow)
				}
			}
			if tc.checkBody != nil {
				tc.checkBody(t, rr.Body.String())
			}
			if tc.checkCalls != nil {
				tc.checkCalls(t, env.server, env.sess.Id())
			}
		})
	}
}
