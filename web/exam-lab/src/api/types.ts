// Wire and domain types for the /api/examdocs endpoint. These mirror the Go
// types in pkg/api/exam/exam.go and pkg/models/question/question.go. The Go
// structs carry no json tags, so their fields marshal under their capitalized
// names.

// ExamCategory mirrors the Go question.ExamCategory string enum
// (pkg/models/question/question.go): a proctored certification exam or an
// unproctored practice exam.
export type ExamCategory = "certification-exam" | "practice-exam";

// ExamCategoryLabels is the human-readable label for each ExamCategory,
// for display wherever the raw wire value would be shown.
export const ExamCategoryLabels: Record<ExamCategory, string> = {
  "certification-exam": "Certification Exam",
  "practice-exam": "Practice Exam",
};

// ExamExcerpt mirrors the Go question.ExamExcerpt projection. Title and
// Description are question.Plaintext, which is `type PlainText string`, so they
// marshal as plain strings here.
export type ExamExcerpt = {
  Id: string;
  ShortName: string;
  Code: string;
  Title: string;
  Description: string;
  ExamCategory: ExamCategory;
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
// fetched. current_question is that question itself; the two are coherent, so
// current_question is non-null exactly when current_question_index >= 0.
export type ExamSessionSummary = {
  exam_session_id: string;
  exam_excerpt: ExamExcerpt;
  started_at: number;
  // options is the examserver.ExamOptions bitmask the session was created
  // with; test its bits with the ExamOption* constants below (e.g. a session
  // only supports seeking when (options & ExamOptionSeekable) !== 0).
  options: number;
  current_question_index: number;
  current_question: Question | null;
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

// ExamReport mirrors the Go examreport.ExamReport struct
// (pkg/models/examreport/examreport.go): the full report recorded by the
// tracking server when an exam session is finished. Its json tags produce
// camelCase wire fields. finishedAt is a millisecond-resolution unix
// timestamp, so `new Date(finishedAt)` works directly. examShortName,
// examCode, description, and passingScore are omitted from the wire when
// empty. assessment reuses the question.Assessment shape defined above
// (certification-exam reports omit its questions, practice-exam reports
// include them for review).
export type ExamReport = {
  id: string;
  examTaker: {
    persons?: { name: string; fistname?: string; lastname?: string }[];
    anonymous?: { sessionId: string }[];
  };
  examId: string;
  examShortName?: string;
  examCode?: string;
  title: string;
  description?: string;
  passingScore?: number;
  examCategory: ExamCategory;
  examSessionId: string;
  finishedAt: number;
  assessment: Assessment;
};

// ExamTrackingListResponse is the JSON body of a successful
// GET /api/examtrackings: {"exam_reports": [{...}, ...]}.
export type ExamTrackingListResponse = {
  exam_reports: ExamReport[];
};

// ExamReports is the resolved list of reports exposed by useExamTrackings.
export type ExamReports = ExamReport[];

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

// CLIENT_SUPPORTED_QUESTION_TYPES lists the question types the client can
// render; drag-and-drop exists on the wire but has no renderer yet. Sessions
// created without user-chosen types are restricted to these.
export const CLIENT_SUPPORTED_QUESTION_TYPES: QuestionType[] = [
  "single-choice",
  "multiple-choice",
];

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

// OverallResult mirrors the Go question.OverallResult string enum
// (pkg/models/question/question.go): the overall verdict for a graded
// submission. "pass" means the earned score reached the passing score;
// "immediate" is any non-passing result returned right away.
export type OverallResult = "pass" | "immediate";

// ScoreResult mirrors the Go question.ScoreResult
// {earnedScore, totalScore}: the aggregate earned and total scores for a
// graded submission.
export type ScoreResult = {
  earnedScore: number;
  totalScore: number;
};

// QuestionScore mirrors the Go question.QuestionScore
// {questionId, scoreEarned}: the score earned on a single question.
export type QuestionScore = {
  questionId: string;
  scoreEarned: number;
};

// AssessedQuestion is a Question as embedded in an Assessment: for
// practice-exam submissions the grader attaches the origin question document,
// which carries its correctAnswer so the candidate can review what they got
// wrong. Question deliberately does not model correctAnswer — it must never
// influence the client while answering — so the field is modeled here, where
// revealing it is the point. Certification-exam assessments omit questions
// entirely.
export type AssessedQuestion = Question & {
  correctAnswer?: { options?: QuestionOption[] };
};

// Assessment mirrors the Go question.Assessment struct
// (pkg/models/question/question.go): the score report produced after a
// submission is graded. Its json tags produce camelCase wire fields.
//
// questions holds the original question documents, but only for practice-exam
// category submissions: per the XSD, the origin question (and therefore its
// correct answer) is included so the candidate can review incorrect answers,
// while certification-exam omits it. Only questions the candidate actually
// answered are present.
export type Assessment = {
  overallResult?: OverallResult;
  scoreResult?: ScoreResult;
  questionScores?: QuestionScore[];
  questions?: AssessedQuestion[];
};

// Answer mirrors the Go question.Answer struct (pkg/models/question/question.go):
// a candidate's response to a single question. For single-choice and
// multiple-choice questions it carries the selected options; drag-and-drop
// connections are not modeled because the client cannot render that type yet.
export type Answer = {
  questionId: string;
  questionType: QuestionType;
  options?: QuestionOption[];
};

// ExamAnswer mirrors the Go question.ExamAnswer struct: a complete submission
// carrying one answer per answered question. It is exam-scoped — the server
// keeps a single ExamAnswer per session and replaces it wholesale on every
// persisted submission — so each submit merges the current question's answer
// into the previously saved one rather than posting it alone.
export type ExamAnswer = {
  answers?: Answer[];
};

// MyAnswerResponse is the JSON body of a successful
// GET /api/examsessions/{exam_session_id}/my_answer:
// {"exam_answer": {...} or null}; null when no answer has been submitted yet.
export type MyAnswerResponse = {
  exam_answer: ExamAnswer | null;
};

// SubmitAnswerResponse is the JSON body of a successful
// POST /api/examsessions/{exam_session_id}/answer[?check_only=true]:
// {"assessment": {...}}. With check_only=true the answer is graded but not
// persisted; otherwise it is saved as the session's latest submission.
export type SubmitAnswerResponse = {
  assessment: Assessment | null;
};
