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
	startID     examserver.ExamId
	startedExam *question.Exam
	startedUser string
	startedOpts examserver.ExamOptions

	listResult []examserver.ExamId
	listCalls  []string

	endErr error
	ended  []examserver.ExamId
}

func (s *fakeExamServer) StartNewExamSession(_ context.Context, exam *question.Exam, userSessionId string, opts examserver.ExamOptions) (examserver.ExamId, error) {
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

func (s *fakeExamServer) ListExamSessions(_ context.Context, userSessionId string) []examserver.ExamId {
	s.listCalls = append(s.listCalls, userSessionId)
	return s.listResult
}

func (s *fakeExamServer) EndExamSession(_ context.Context, examId examserver.ExamId) error {
	s.ended = append(s.ended, examId)
	return s.endErr
}

func (s *fakeExamServer) GetNextQuestion(context.Context, examserver.ExamId, *examserver.QuestionCursor) (*question.Question, *examserver.QuestionCursor, error) {
	return nil, nil, nil
}

func (s *fakeExamServer) SeekCursorTo(context.Context, examserver.ExamId, *examserver.QuestionCursor, int) (*examserver.QuestionCursor, error) {
	return nil, nil
}

func (s *fakeExamServer) SubmitAnswer(context.Context, examserver.ExamId, string, bool) (string, error) {
	return "", nil
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
	mux.Handle("/api/examsessions/{exam_id}", h)
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
			name:       "list success",
			method:     http.MethodGet,
			target:     "/api/examsessions",
			server:     &fakeExamServer{listResult: []examserver.ExamId{"a", "b"}},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				var got struct {
					ExamSessionIDs []string `json:"exam_session_ids"`
				}
				if err := json.Unmarshal([]byte(body), &got); err != nil {
					t.Fatalf("decode body %q: %v", body, err)
				}
				want := []string{"a", "b"}
				if len(got.ExamSessionIDs) != len(want) {
					t.Fatalf("exam_session_ids = %v, want %v", got.ExamSessionIDs, want)
				}
				for i := range want {
					if got.ExamSessionIDs[i] != want[i] {
						t.Errorf("exam_session_ids[%d] = %q, want %q", i, got.ExamSessionIDs[i], want[i])
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
					ExamSessionIDs []string `json:"exam_session_ids"`
				}
				if err := json.Unmarshal([]byte(body), &got); err != nil {
					t.Fatalf("decode body %q: %v", body, err)
				}
				if len(got.ExamSessionIDs) != 0 {
					t.Errorf("exam_session_ids = %v, want empty", got.ExamSessionIDs)
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
