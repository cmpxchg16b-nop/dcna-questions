// Wire and domain types for the /api/examdocs endpoint. These mirror the Go
// types in pkg/api/exam/exam.go and pkg/models/question/question.go. The Go
// structs carry no json tags, so their fields marshal under their capitalized
// names.

// ExamExcerpt mirrors the Go question.ExamExcerpt projection. Title and
// Description are question.Plaintext, which is `type PlainText string`, so they
// marshal as plain strings here.
export type ExamExcerpt = {
  Id: string;
  ShortName: string;
  Code: string;
  Title: string;
  Description: string;
  NumQuestions: number;
  TotalScores: number;
};

// ExamDocs is the resolved list of exam excerpts exposed by useExamDocs.
export type ExamDocs = ExamExcerpt[];

// One streamed NDJSON line from /api/examdocs: exactly one of Data or Err is
// set. Maps to the Go ndjsonLine struct {"Err":"...","Data":{...}}.
export type ExamDocsLine = { Data?: ExamExcerpt; Err?: string };

// ExamSessionSummary mirrors the Go examSessionSummary struct in
// pkg/api/examsessions/examsession.go. Unlike ExamDocumentExcerpt it carries
// json tags, so its wire fields are snake_case. started_at is a
// millisecond-resolution unix timestamp, so `new Date(started_at)` works
// directly. exam_excerpt is a question.ExamDocumentExcerpt, which marshals
// under capitalized field names just like in the examdocs endpoint.
// current_question_index is the virtual index of the question most recently
// served by GetNextQuestion; it is -1 before the first question has been
// fetched.
export type ExamSessionSummary = {
  exam_session_id: string;
  exam_excerpt: ExamExcerpt;
  started_at: number;
  // options is the examserver.ExamOptions bitmask the session was created
  // with; test its bits with the ExamOption* constants below (e.g. a session
  // only supports seeking when (options & ExamOptionSeekable) !== 0).
  options: number;
  current_question_index: number;
};

// ExamSessionListResponse is the JSON body of a successful
// GET /api/examsessions: {"exam_sessions": [{...}, ...]}.
export type ExamSessionListResponse = {
  exam_sessions: ExamSessionSummary[];
};

// ExamSessionResponse is the JSON body of a successful
// GET /api/examsessions/{exam_session_id}: {"exam_session": {...}}.
export type ExamSessionResponse = {
  exam_session: ExamSessionSummary;
};

// ExamOptions bitmask bits, mirroring the Go examserver.ExamOptions constants
// in pkg/models/examserver/examserver.go. The API accepts them combined as the
// "options" field of POST /api/examsessions.
export const ExamOptionRandomQuestions = 1 << 0; // randomized questions ordering within a collection
export const ExamOptionRandomOptions = 1 << 1; // randomized options ordering
export const ExamOptionSeekable = 1 << 2; // allows seeking to a question by index at will
export const ExamOptionRandomQuestionColl = 1 << 3; // randomized question collection picking

// QuestionType mirrors the Go question.QuestionType string enum
// (pkg/models/question/question.go).
export type QuestionType =
  "single-choice" | "multiple-choice" | "drag-and-drop";

// CreateExamSessionRequest is the JSON body of POST /api/examsessions:
// {"exam_id": "...", "options": <bitmask>, "accept_question_types": [...]}.
// options defaults to 0 (document order, not seekable) when absent;
// accept_question_types restricts which question types the session serves
// (absent or empty accepts every type).
export type CreateExamSessionRequest = {
  exam_id: string;
  options?: number;
  accept_question_types?: QuestionType[];
};

// CreateExamSessionResponse is the JSON body of a successful
// POST /api/examsessions: {"exam_session_id": "..."}.
export type CreateExamSessionResponse = {
  exam_session_id: string;
};

// ExamSessions is the resolved list of session summaries exposed by
// useExamSessions.
export type ExamSessions = ExamSessionSummary[];

// QuestionOption mirrors the Go question.Option struct {"id", "content"}.
export type QuestionOption = {
  id: string;
  content: string;
};

// Question mirrors the subset of the Go question.Question struct
// (pkg/models/question/question.go) that the client renders. Its json tags
// produce camelCase wire fields. correctAnswer is deliberately not modeled:
// it is part of the wire payload but must never influence the client.
export type Question = {
  id: string;
  type: QuestionType;
  score?: number;
  description: { text: string };
  exhibits?: { image: { src: string } }[];
  options?: QuestionOption[];
};

// NextQuestionResponse is the JSON body of a successful
// GET /api/examsessions/{exam_session_id}/questions?cursor_id=<cursor>:
// {"cursor_id": <next or null>, "question": {...} or null}. cursor_id is the
// opaque token to continue forward with; both fields are null once the session
// has no more questions.
export type NextQuestionResponse = {
  cursor_id: string | null;
  question: Question | null;
};

// SeekCursorResponse is the JSON body of a successful
// PUT /api/examsessions/{exam_session_id}/cursors?cursor_id=<cursor>&index=<n>:
// {"cursor_id": "..."}. The returned cursor must be used for the next read;
// any cursor passed in is invalidated by the seek.
export type SeekCursorResponse = {
  cursor_id: string;
};
