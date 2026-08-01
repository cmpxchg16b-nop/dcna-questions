package examdocs_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	examdocs "dcna-questions/pkg/api/examdocs"
	"dcna-questions/pkg/models/question"
)

// fakeLoader is an ExamLoader that serves canned exams by URL and can be wired
// to fail for specific URLs, so tests can exercise both the Data and Err lines
// of the NDJSON stream.
type fakeLoader struct {
	byURL  map[string]*question.Exam
	errURL map[string]bool
}

func (l *fakeLoader) LoadFrom(url string) (*question.Exam, error) {
	if l.errURL[url] {
		return nil, errors.New("disk read failed")
	}
	if e, ok := l.byURL[url]; ok {
		return e, nil
	}
	return nil, errors.New("exam not found")
}

// ndLine mirrors the handler's on-the-wire ndjsonLine so tests can decode each
// streamed line. Data decodes straight back into question.ExamExcerpt because
// ExamExcerpt's exported fields carry no JSON tags.
type ndLine struct {
	Err  string                `json:"Err,omitempty"`
	Data *question.ExamDocumentExcerpt `json:"Data,omitempty"`
}

// examWith builds a minimal exam whose first (and only) question collection
// holds one single-choice question per score; ExamExcerptFrom therefore reports
// NumQuestions = len(scores) and TotalScores = sum(scores).
func examWith(id, code string, scores ...int) *question.Exam {
	qs := make([]question.Question, len(scores))
	for i, s := range scores {
		qs[i] = question.Question{
			Id:    fmt.Sprintf("%s-q%d", id, i+1),
			Type:  question.QuestionTypeSingleChoice,
			Score: s,
		}
	}
	return &question.Exam{
		Id:        id,
		ShortName: id,
		Code:      code,
		Title:     question.PlainText("Title " + id),
		QuestionSet: question.QuestionSet{
			QuestionCollections: []question.QuestionCollection{{Questions: qs}},
		},
	}
}

// parseLines splits an NDJSON body into decoded lines, skipping blanks.
func parseLines(t *testing.T, body string) []ndLine {
	t.Helper()
	var lines []ndLine
	for _, raw := range strings.Split(body, "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var l ndLine
		if err := json.Unmarshal([]byte(raw), &l); err != nil {
			t.Fatalf("unmarshal ndjson line %q: %v", raw, err)
		}
		lines = append(lines, l)
	}
	return lines
}

func TestExamHandler(t *testing.T) {
	tests := []struct {
		name            string
		method          string
		sources         []question.ExamSource
		wantStatus      int
		wantContentType string
		check           func(t *testing.T, rr *httptest.ResponseRecorder)
	}{
		{
			name:   "GET streams exams as NDJSON excerpts",
			method: http.MethodGet,
			sources: []question.ExamSource{
				{
					Loader: &fakeLoader{byURL: map[string]*question.Exam{
						"u1": examWith("A", "cA", 1),
						"u2": examWith("B", "cB", 1, 2),
					}},
					URLs: []string{"u1", "u2"},
				},
			},
			wantStatus:      http.StatusOK,
			wantContentType: "application/x-ndjson",
			check: func(t *testing.T, rr *httptest.ResponseRecorder) {
				if got := rr.Header().Get("X-Accel-Buffering"); got != "no" {
					t.Errorf("X-Accel-Buffering = %q, want no", got)
				}
				lines := parseLines(t, rr.Body.String())
				if len(lines) != 2 {
					t.Fatalf("got %d lines, want 2 (body %q)", len(lines), rr.Body.String())
				}
				cases := []struct {
					id         string
					num, total int
				}{
					{"A", 1, 1},
					{"B", 2, 3},
				}
				for i, c := range cases {
					if lines[i].Err != "" {
						t.Errorf("line %d: unexpected Err %q", i, lines[i].Err)
					}
					if lines[i].Data == nil {
						t.Fatalf("line %d: nil Data", i)
					}
					if lines[i].Data.Id != c.id || lines[i].Data.NumQuestions != c.num || lines[i].Data.TotalScores != c.total {
						t.Errorf("line %d: Data = %+v, want {Id:%s NumQuestions:%d TotalScores:%d}",
							i, lines[i].Data, c.id, c.num, c.total)
					}
				}
			},
		},
		{
			name:   "GET reports load failures as in-band Err lines",
			method: http.MethodGet,
			sources: []question.ExamSource{
				{
					Loader: &fakeLoader{
						byURL:  map[string]*question.Exam{"ok": examWith("A", "cA", 1)},
						errURL: map[string]bool{"bad": true},
					},
					URLs: []string{"ok", "bad"},
				},
			},
			wantStatus:      http.StatusOK,
			wantContentType: "application/x-ndjson",
			check: func(t *testing.T, rr *httptest.ResponseRecorder) {
				lines := parseLines(t, rr.Body.String())
				if len(lines) != 2 {
					t.Fatalf("got %d lines, want 2 (body %q)", len(lines), rr.Body.String())
				}
				if lines[0].Data == nil || lines[0].Data.Id != "A" {
					t.Errorf("line 0 Data = %+v, want exam A", lines[0].Data)
				}
				if lines[0].Err != "" {
					t.Errorf("line 0: unexpected Err %q", lines[0].Err)
				}
				if lines[1].Data != nil {
					t.Errorf("line 1 Data = %+v, want nil", lines[1].Data)
				}
				// The repository wraps the load error with the offending URL.
				if !strings.Contains(lines[1].Err, "bad") {
					t.Errorf("line 1 Err = %q, want it to mention the failing URL %q", lines[1].Err, "bad")
				}
			},
		},
		{
			name:            "GET with no sources streams an empty body",
			method:          http.MethodGet,
			sources:         nil,
			wantStatus:      http.StatusOK,
			wantContentType: "application/x-ndjson",
			check: func(t *testing.T, rr *httptest.ResponseRecorder) {
				if strings.TrimSpace(rr.Body.String()) != "" {
					t.Errorf("got body %q, want empty", rr.Body.String())
				}
			},
		},
		{
			name:            "non-GET responds 405",
			method:          http.MethodPost,
			sources:         nil,
			wantStatus:      http.StatusMethodNotAllowed,
			wantContentType: "text/plain",
			check: func(t *testing.T, rr *httptest.ResponseRecorder) {
				if !strings.Contains(rr.Body.String(), "method not allowed") {
					t.Errorf("got body %q, want it to mention method not allowed", rr.Body.String())
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := examdocs.NewExamHandler(question.NewExamRepository(tc.sources))
			rr := httptest.NewRecorder()
			r := httptest.NewRequest(tc.method, "/api/examdocs", nil)

			h.ServeHTTP(rr, r)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", rr.Code, tc.wantStatus, rr.Body.String())
			}
			if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, tc.wantContentType) {
				t.Errorf("Content-Type = %q, want it to contain %q", ct, tc.wantContentType)
			}
			if tc.check != nil {
				tc.check(t, rr)
			}
		})
	}
}

// failingWriter is an http.ResponseWriter whose Write always errors, simulating
// a client that has disconnected mid-stream. It also implements http.Flusher.
type failingWriter struct {
	header http.Header
}

func (w *failingWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}
func (w *failingWriter) Write([]byte) (int, error) { return 0, errors.New("client gone") }
func (w *failingWriter) WriteHeader(int)           {}
func (w *failingWriter) Flush()                    {}

// TestExamHandler_ClientDisconnect exercises the drain path: when the writer
// fails, ServeHTTP must keep consuming ListExamDocuments' unbuffered channel so
// the producer goroutine is not left blocked, and must return rather than hang.
func TestExamHandler_ClientDisconnect(t *testing.T) {
	repo := question.NewExamRepository([]question.ExamSource{
		{
			Loader: &fakeLoader{byURL: map[string]*question.Exam{
				"u1": examWith("A", "cA", 1),
				"u2": examWith("B", "cB", 1),
			}},
			URLs: []string{"u1", "u2"},
		},
	})
	h := examdocs.NewExamHandler(repo)
	r := httptest.NewRequest(http.MethodGet, "/api/examdocs", nil)

	done := make(chan struct{})
	go func() {
		h.ServeHTTP(&failingWriter{}, r)
		close(done)
	}()
	select {
	case <-done:
		// ServeHTTP returned despite every Write failing: the handler drained the
		// stream instead of blocking on the broken writer or the producer.
	case <-time.After(2 * time.Second):
		t.Fatal("ServeHTTP hung after client disconnect; stream was not drained")
	}
}
