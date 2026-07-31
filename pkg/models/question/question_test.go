package question

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestExamLoader_LoadDecodesNumericEntities(t *testing.T) {
	// The question bank uses numeric character references (e.g. &#8226; for the
	// bullet). These must resolve without any custom entity table.
	const xml = `<?xml version="1.0" encoding="UTF-8"?>
<root>
<exam id="1" shortname="DCACI" code="300-620">
  <title>Implementing Cisco ACI</title>
  <description>x</description>
  <questionset>
    <questioncollection>
      <question id="1" num="1" type="single-choice">
        <description>step &#8226; one</description>
        <options><option id="1">a</option></options>
        <correctanswer><options><option id="1">a</option></options></correctanswer>
      </question>
    </questioncollection>
  </questionset>
</exam>
</root>`

	exam, err := NewFileExamLoader().Load([]byte(xml))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	desc := string(exam.QuestionSet.QuestionCollections[0].Questions[0].Description.Text)
	if !strings.Contains(desc, "•") {
		t.Fatalf("bullet not decoded, got %q", desc)
	}
}

func TestExamLoader_LoadMultipleCollections(t *testing.T) {
	// Regression guard: confirms the standard xml decoder populates a slice of
	// QuestionCollection structs (each a <questioncollection>) without the old
	// custom UnmarshalXML that existed only to handle a slice-of-slices.
	const xml = `<?xml version="1.0" encoding="UTF-8"?>
<root>
<exam id="1" shortname="X" code="1">
  <title>t</title><description>d</description>
  <questionset>
    <questioncollection>
      <question id="1" num="1" type="single-choice"><description>a</description></question>
      <question id="2" num="2" type="single-choice"><description>b</description></question>
    </questioncollection>
    <questioncollection>
      <question id="3" num="3" type="multiple-choice"><description>c</description></question>
    </questioncollection>
  </questionset>
</exam>
</root>`

	exam, err := NewFileExamLoader().Load([]byte(xml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(exam.QuestionSet.QuestionCollections) != 2 {
		t.Fatalf("expected 2 collections, got %d", len(exam.QuestionSet.QuestionCollections))
	}
	if len(exam.QuestionSet.QuestionCollections[0].Questions) != 2 {
		t.Fatalf("expected 2 questions in collection 0, got %d",
			len(exam.QuestionSet.QuestionCollections[0].Questions))
	}
	if exam.QuestionSet.QuestionCollections[1].Questions[0].Id != "3" {
		t.Fatalf("expected collection 1 q0 id=3, got %q",
			exam.QuestionSet.QuestionCollections[1].Questions[0].Id)
	}
}

func TestExamLoader_LoadRejectsUnknownQuestionType(t *testing.T) {
	const xml = `<?xml version="1.0" encoding="UTF-8"?>
<root>
<exam id="1" shortname="DCACI" code="300-620">
  <title>t</title><description>d</description>
  <questionset>
    <questioncollection>
      <question id="1" num="1" type="bogus-type">
        <description>x</description>
      </question>
    </questioncollection>
  </questionset>
</exam>
</root>`

	if _, err := NewFileExamLoader().Load([]byte(xml)); err == nil {
		t.Fatal("Load: expected error for unknown question type, got nil")
	}
}

func TestExamLoader_LoadRejectsMissingExamId(t *testing.T) {
	const xml = `<?xml version="1.0" encoding="UTF-8"?>
<root>
<exam shortname="DCACI" code="300-620">
  <title>t</title><description>d</description>
  <questionset><questioncollection></questioncollection></questionset>
</exam>
</root>`

	_, err := NewFileExamLoader().Load([]byte(xml))
	if err == nil || !strings.Contains(err.Error(), "missing exam id") {
		t.Fatalf("Load: expected missing-exam-id error, got %v", err)
	}
}

func TestExamLoader_LoadFileMissing(t *testing.T) {
	if _, err := NewFileExamLoader().LoadFile("does-not-exam.xml"); err == nil {
		t.Fatal("LoadFile: expected error for missing file, got nil")
	}
}

// TestExamLoader_LoadFileRealRepoExam loads the real exam1.xml at the repo root,
// exercising the full nested <questionset>><questioncollection>><question>
// structure end-to-end.
func TestExamLoader_LoadFileRealRepoExam(t *testing.T) {
	exam, err := NewFileExamLoader().LoadFile(filepath.Join("..", "..", "..", "exam1.xml"))
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if exam.Id != "1" || exam.ShortName != "DCNA" {
		t.Fatalf("unexpected exam meta: %+v", exam)
	}
	qs := exam.QuestionSet.QuestionCollections
	if len(qs) != 1 || len(qs[0].Questions) != 5 {
		t.Fatalf("expected 1 collection with 5 questions, got %d collections", len(qs))
	}
	// Question 3 is the drag-and-drop with a bullet (&#8226;) in its description.
	q3 := qs[0].Questions[2]
	if q3.Type != QuestionTypeDragAndDrop {
		t.Fatalf("expected q3 drag-and-drop, got %q", q3.Type)
	}
	if !strings.Contains(string(q3.Description.Text), "•") {
		t.Fatalf("expected bullet in q3 description, got %q", q3.Description.Text)
	}
}

func TestQuestionType_Valid(t *testing.T) {
	for _, tc := range []struct {
		t   QuestionType
		exp bool
	}{
		{QuestionTypeSingleChoice, true},
		{QuestionTypeMultipleChoice, true},
		{QuestionTypeDragAndDrop, true},
		{QuestionType("nonsense"), false},
		{QuestionType(""), false},
	} {
		if got := tc.t.Valid(); got != tc.exp {
			t.Errorf("Valid(%q) = %v, want %v", tc.t, got, tc.exp)
		}
	}
}
