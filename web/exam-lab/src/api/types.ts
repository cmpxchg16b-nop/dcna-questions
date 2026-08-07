// Wire and domain types for the /api/examdocs endpoint. These mirror the Go
// types in pkg/api/exam/exam.go and pkg/models/question/question.go. The Go
// structs carry no json tags, so their fields marshal under their capitalized
// names.

// ExamCategory mirrors the Go question.ExamCategory string enum
// (pkg/models/question/question.go): a proctored certification exam or an
// unproctored practice exam.
export type ExamCategory = "certification-exam" | "practice-exam";

// The human-readable label for each category lives in the i18n bundles
// (src/i18n/locales) under "exam.category", keyed by the wire value.

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

// ProfileResponse is the JSON body of a successful GET /api/profile:
// {"session_id": "...", "subject_id": "..."}. Mirrors the Go ProfileResponse
// struct in pkg/api/profile/profile.go.
export type ProfileResponse = {
  session_id: string;
  subject_id: string;
};

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
// render. Sessions created without user-chosen types are restricted to these.
export const CLIENT_SUPPORTED_QUESTION_TYPES: QuestionType[] = [
  "single-choice",
  "multiple-choice",
  "drag-and-drop",
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

// DragCandidate mirrors the Go question.Candidate struct {"id", "content"}: a
// draggable source item of a drag-and-drop question. Ids are unique within the
// question scope.
export type DragCandidate = {
  id: string;
  content: string;
};

// DropTarget mirrors the Go question.Drop struct {"id", "content"}: one drop
// slot of a drag-and-drop question. Its content is the slot's label and may be
// empty — e.g. the anonymous slots of a multi-area drop, whose meaning comes
// from the enclosing area's label.
export type DropTarget = {
  id: string;
  content: string;
};

// DropArea mirrors the Go question.DropArea struct {"id", "label", "drops"}:
// one labeled sub-section of a multi-area drop zone. Drop ids share the
// question-wide drop id namespace even across areas, so a connection's dst is
// unique no matter which area the drop belongs to.
export type DropArea = {
  id: string;
  label?: string;
  drops: DropTarget[];
};

// MultiAreaDrop mirrors the Go question.MultiAreaDrop struct {"dropAreas"}:
// the drop zone of a drag-and-drop question when it is split into labeled
// sub-sections, in contrast to the flat drops list. A question carries exactly
// one of the two.
export type MultiAreaDrop = {
  dropAreas: DropArea[];
};

// ImgCandidate mirrors the Go question.ImgCandidate struct: a draggable image
// snippet of an image-based drag-and-drop question, identified by nodeId.
// nodeLabel is the descriptive text used when rendering the correct answer.
// width/height give the snippet's intrinsic pixel size.
export type ImgCandidate = {
  nodeId: string;
  nodeLabel: string;
  width: number;
  height: number;
  imgDataSrc: string;
};

// ImgDrop mirrors the Go question.ImgDrop struct: a single drop target laid out
// at an absolute position over the drop area's background image. nodeLabel is
// the descriptive text used when rendering the correct answer.
export type ImgDrop = {
  nodeId: string;
  nodeLabel: string;
  positionX: number;
  positionY: number;
  width: number;
  height: number;
};

// ImgDropsArea mirrors the Go question.ImgDropsArea struct: the background image
// that hosts the positioned drop targets of an image-based drag-and-drop
// question.
export type ImgDropsArea = {
  imgBackgroundUrl: string;
  width: number;
  height: number;
  imgDrops: ImgDrop[];
};

// ImgDragAndDrop mirrors the Go question.ImgDragAndDrop struct: an image-based
// drag-and-drop question where the candidate drags image snippets onto
// absolutely positioned drop targets over a background image. It is an
// alternative payload to the candidate/drop lists for a drag-and-drop question.
export type ImgDragAndDrop = {
  imgCandidates: ImgCandidate[];
  imgDropsArea: ImgDropsArea;
};

// Connect mirrors the Go question.Connect struct {"src", "dst"}: one
// connection in a drag-and-drop answer, from the candidate src onto the drop
// dst.
export type Connect = {
  src: string;
  dst: string;
};

// ConnectEndpoint mirrors the Go question.ConnectSource and
// question.ConnectDestination structs, both {"id"}.
export type ConnectEndpoint = {
  id: string;
};

// ConnectCombination mirrors the Go question.ConnectCombination struct
// {"connectSources", "connectDestinations"}: the Cartesian product of its
// sources and destinations yields the set of connections it accepts.
export type ConnectCombination = {
  connectSources?: ConnectEndpoint[];
  connectDestinations?: ConnectEndpoint[];
};

// ConnectionSolution mirrors the Go question.ConnectionSolution struct: one
// acceptable correct answer for a drag-and-drop question. A submission
// satisfies it by making at least requiredUniqueConnections unique
// connections, each drawn from its explicit connects or the products of its
// connect combinations. A question with several solutions is correct when any
// one of them is satisfied.
export type ConnectionSolution = {
  requiredUniqueConnections: number;
  connects?: Connect[];
  connectCombinations?: ConnectCombination[];
};

// Question mirrors the subset of the Go question.Question struct
// (pkg/models/question/question.go) that the client renders. Its json tags
// produce camelCase wire fields. correctAnswer is deliberately not modeled:
// the exam server strips it from every served question, so the only place it
// ever appears on the wire is inside an assessment (see AssessedQuestion).
export type Question = {
  id: string;
  type: QuestionType;
  score?: number;
  description: { text: string };
  exhibits?: { image: { src: string } }[];
  options?: QuestionOption[];
  // Drag-and-drop payload: candidates are the draggable source items, and the
  // drop zone is either the flat drops list or a multiAreaDrop split into
  // labeled sub-sections (exactly one of the two is present).
  candidates?: DragCandidate[];
  multiAreaDrop?: MultiAreaDrop;
  drops?: DropTarget[];
  // Image-based drag-and-drop payload: an alternative to the candidate/drop
  // lists above, present when the drag-and-drop question is image-based. When
  // set, the candidate/drop lists are absent and the board renders image
  // snippets over a positioned background.
  imgDragAndDrop?: ImgDragAndDrop;
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
  correctAnswer?: {
    options?: QuestionOption[];
    connectionSolutions?: ConnectionSolution[];
  };
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
// multiple-choice questions it carries the selected options; for drag-and-drop
// questions it carries the placed connections.
export type Answer = {
  questionId: string;
  questionType: QuestionType;
  options?: QuestionOption[];
  connections?: Connect[];
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

// UserUploadSummary mirrors the Go summaryDTO struct in
// pkg/api/useruploads/useruploads.go: the metadata of one file the caller has
// uploaded. last_modified_at is a Unix millisecond timestamp, so
// `new Date(last_modified_at)` works directly.
export type UserUploadSummary = {
  upload_id: string;
  filename: string;
  mime_type: string;
  size_bytes: number;
  last_modified_at: number;
  sha256: string;
  user_id: string;
};

// UserUploadListResponse is the JSON body of a successful
// GET /api/useruploads: {"uploads": [{...}, ...]}.
export type UserUploadListResponse = {
  uploads: UserUploadSummary[];
};

// UserUploads is the resolved list of upload summaries exposed by useUploads.
export type UserUploads = UserUploadSummary[];

// ExamAssociation mirrors the Go associationDTO struct in
// pkg/api/examassociations/examassociations.go: a binding between one of the
// caller's uploads and the exam documents contained in it. The server keeps at
// most one association per upload, so upload_id is unique within the list.
export type ExamAssociation = {
  id: string;
  user_id: string;
  upload_id: string;
};

// ExamAssociationListResponse is the JSON body of a successful
// GET /api/examassociations: {"associations": [{...}, ...]}.
export type ExamAssociationListResponse = {
  associations: ExamAssociation[];
};

// ExamAssociations is the resolved list of associations exposed by
// useExamAssociations.
export type ExamAssociations = ExamAssociation[];
