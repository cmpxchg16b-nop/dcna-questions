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
