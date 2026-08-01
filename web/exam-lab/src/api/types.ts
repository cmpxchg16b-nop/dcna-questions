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

// CreateExamSessionRequest is the JSON body of POST /api/examsessions:
// {"exam_id": "...", "options": <bitmask>}. options defaults to 0 (document
// order, not seekable) when absent.
export type CreateExamSessionRequest = {
  exam_id: string;
  options?: number;
};

// CreateExamSessionResponse is the JSON body of a successful
// POST /api/examsessions: {"exam_session_id": "..."}.
export type CreateExamSessionResponse = {
  exam_session_id: string;
};

// ExamSessions is the resolved list of session summaries exposed by
// useExamSessions.
export type ExamSessions = ExamSessionSummary[];
