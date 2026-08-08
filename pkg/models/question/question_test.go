package question

import (
	"context"
	"encoding/xml"
	"os"
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
  <examcategory>certification-exam</examcategory>
  <questionset>
    <questioncollection>
      <question id="1" type="single-choice">
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
  <examcategory>certification-exam</examcategory>
  <questionset>
    <questioncollection>
      <question id="1" type="single-choice"><description>a</description></question>
      <question id="2" type="single-choice"><description>b</description></question>
    </questioncollection>
    <questioncollection>
      <question id="3" type="multiple-choice"><description>c</description></question>
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
  <examcategory>certification-exam</examcategory>
  <questionset>
    <questioncollection>
      <question id="1" type="bogus-type">
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
  <examcategory>certification-exam</examcategory>
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
	if exam.Id != "example-1" || exam.ShortName != "DCNA" {
		t.Fatalf("unexpected exam meta: %+v", exam)
	}
	qs := exam.QuestionSet.QuestionCollections
	if len(qs) != 1 || len(qs[0].Questions) != 4 {
		t.Fatalf("expected 1 collection with 4 questions, got %d collections", len(qs))
	}
	// Question 7 is the image-based drag-and-drop question.
	q7 := qs[0].Questions[3]
	if q7.Id != "7" || q7.Type != QuestionTypeDragAndDrop {
		t.Fatalf("expected q7 drag-and-drop, got id %q type %q", q7.Id, q7.Type)
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

func TestExamCategory_Valid(t *testing.T) {
	for _, tc := range []struct {
		c   ExamCategory
		exp bool
	}{
		{ExamCategoryCertification, true},
		{ExamCategoryPractice, true},
		{ExamCategory("nonsense"), false},
		{ExamCategory(""), false},
	} {
		if got := tc.c.Valid(); got != tc.exp {
			t.Errorf("Valid(%q) = %v, want %v", tc.c, got, tc.exp)
		}
	}
}

func TestExamLoader_LoadRejectsUnknownExamCategory(t *testing.T) {
	const xml = `<?xml version="1.0" encoding="UTF-8"?>
<root>
<exam id="1" shortname="DCACI" code="300-620">
  <title>t</title><description>d</description>
  <examcategory>bogus-category</examcategory>
  <questionset><questioncollection></questioncollection></questionset>
</exam>
</root>`

	_, err := NewFileExamLoader().Load([]byte(xml))
	if err == nil || !strings.Contains(err.Error(), "unknown exam category") {
		t.Fatalf("Load: expected unknown-exam-category error, got %v", err)
	}
}

// minimalExamXML is a valid, minimal exam document used as file content by the
// exam-source tests.
const minimalExamXML = `<?xml version="1.0" encoding="UTF-8"?>
<root>
<exam id="1" shortname="X" code="1">
  <title>t</title><description>d</description>
  <examcategory>certification-exam</examcategory>
  <questionset><questioncollection>
    <question id="1" type="single-choice"><description>a</description></question>
  </questioncollection></questionset>
</exam>
</root>`

func TestStaticFileExamSource_GetReturnsEntries(t *testing.T) {
	entries := []ExamSourceEntry{
		{Loader: NewFileExamLoader(), URLs: []string{"a.xml", "b.xml"}},
		{Loader: NewFileExamLoader(), URLs: []string{"c.xml"}},
	}
	src := NewStaticFileExamSource(entries)
	got := src.Get()
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if len(got[0].URLs) != 2 || got[0].URLs[0] != "a.xml" {
		t.Fatalf("unexpected entry 0 URLs: %v", got[0].URLs)
	}
	if len(got[1].URLs) != 1 || got[1].URLs[0] != "c.xml" {
		t.Fatalf("unexpected entry 1 URLs: %v", got[1].URLs)
	}
}

func TestStaticFileExamSource_GetEmpty(t *testing.T) {
	src := NewStaticFileExamSource(nil)
	if got := src.Get(); got != nil && len(got) != 0 {
		t.Fatalf("expected nil/empty, got %v", got)
	}
}

func TestDynamicDirExamSource_GetFindsXMLFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "exam1.xml"), []byte(minimalExamXML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "exam2.xml"), []byte(minimalExamXML), 0o644); err != nil {
		t.Fatal(err)
	}
	// Non-XML files and subdirectories must be ignored.
	if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte("ignore me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	src := NewDynamicDirExamSource(dir)
	got := src.Get()
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if len(got[0].URLs) != 2 {
		t.Fatalf("expected 2 URLs, got %d (%v)", len(got[0].URLs), got[0].URLs)
	}
	if got[0].Loader == nil {
		t.Fatal("expected non-nil Loader")
	}
	// Each URL should resolve to a readable, valid exam.
	for _, url := range got[0].URLs {
		exam, err := got[0].Loader.LoadFrom(context.Background(), url)
		if err != nil {
			t.Fatalf("LoadFrom(%q): %v", url, err)
		}
		if exam.Id != "1" {
			t.Fatalf("LoadFrom(%q): expected exam id 1, got %q", url, exam.Id)
		}
	}
}

func TestDynamicDirExamSource_GetEmptyDir(t *testing.T) {
	dir := t.TempDir()
	src := NewDynamicDirExamSource(dir)
	if got := src.Get(); got != nil {
		t.Fatalf("expected nil for empty dir, got %v", got)
	}
}

func TestDynamicDirExamSource_GetMissingDir(t *testing.T) {
	src := NewDynamicDirExamSource(filepath.Join(t.TempDir(), "does-not-exist"))
	if got := src.Get(); got != nil {
		t.Fatalf("expected nil for missing dir, got %v", got)
	}
}

func TestQuestion_MarshalXMLEmitsOnlyPopulatedSections(t *testing.T) {
	// encoding/xml always emits the parent wrapper of a parent>child slice
	// field, even when the slice is empty. The hand-rolled MarshalXML must
	// omit empty optional sections (<exhibits>, <candidates>, <drops>, ...)
	// instead of writing empty wrappers.
	q := Question{
		Id:          "1",
		Type:        QuestionTypeSingleChoice,
		Score:       1,
		Description: QuestionDescription{Text: "stem"},
		Options: Options{
			{Id: "1", Content: "a"},
			{Id: "2", Content: "b"},
		},
		CorrectAnswer: CorrectAnswer{
			Options: Options{{Id: "1", Content: "a"}},
		},
	}

	out, err := xml.Marshal(q)
	if err != nil {
		t.Fatalf("Marshal: unexpected error: %v", err)
	}

	const want = `<question id="1" type="single-choice" score="1">` +
		`<description>stem</description>` +
		`<options><option id="1">a</option><option id="2">b</option></options>` +
		`<correctanswer><options><option id="1">a</option></options></correctanswer>` +
		`</question>`
	if string(out) != want {
		t.Fatalf("marshaled XML mismatch:\n got: %s\nwant: %s", out, want)
	}
}

func TestQuestion_MarshalXMLWithExhibits(t *testing.T) {
	q := Question{
		Id:          "2",
		Type:        QuestionTypeMultipleChoice,
		Score:       1,
		Description: QuestionDescription{Text: "stem"},
		Exhibits:    Exhibits{{Image: Image{Src: "assets/x.png"}}},
		Options:     Options{{Id: "1", Content: "a"}},
		CorrectAnswer: CorrectAnswer{
			Options: Options{{Id: "1", Content: "a"}},
		},
	}

	out, err := xml.Marshal(q)
	if err != nil {
		t.Fatalf("Marshal: unexpected error: %v", err)
	}
	if !strings.Contains(string(out), `<exhibits><exhibit><image src="assets/x.png"></image></exhibit></exhibits>`) {
		t.Fatalf("exhibits missing from marshaled XML: %s", out)
	}

	// The marshaled document must round-trip through the loader's decode path.
	var got Question
	if err := xml.Unmarshal(out, &got); err != nil {
		t.Fatalf("Unmarshal of marshaled question: unexpected error: %v", err)
	}
	if got.Id != q.Id || got.Type != q.Type || got.Score != q.Score {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, q)
	}
	if len(got.Exhibits) != 1 || got.Exhibits[0].Image.Src != "assets/x.png" {
		t.Fatalf("round-trip lost exhibits: %+v", got.Exhibits)
	}
	if len(got.CorrectAnswer.Options) != 1 || got.CorrectAnswer.Options[0].Id != "1" {
		t.Fatalf("round-trip lost correct answer: %+v", got.CorrectAnswer)
	}
}

func TestQuestion_OptionImgSrcRoundTrip(t *testing.T) {
	// The option element's optional imgSrc attribute models a pure-image
	// option; it must survive both the decode path and Question.MarshalXML,
	// and text-only options must not gain an empty imgSrc attribute.
	const examXML = `<?xml version="1.0" encoding="UTF-8"?>
<root>
<exam id="1" shortname="X" code="1">
  <title>t</title><description>d</description>
  <examcategory>certification-exam</examcategory>
  <questionset>
    <questioncollection>
      <question id="1" type="single-choice">
        <description>pick the right topology</description>
        <options>
          <option id="1" imgSrc="assets/opt-a.png"></option>
          <option id="2">text only</option>
        </options>
        <correctanswer><options><option id="1" imgSrc="assets/opt-a.png"></option></options></correctanswer>
      </question>
    </questioncollection>
  </questionset>
</exam>
</root>`

	exam, err := NewFileExamLoader().Load([]byte(examXML))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	q := exam.QuestionSet.QuestionCollections[0].Questions[0]
	if len(q.Options) != 2 || q.Options[0].ImgSrc != "assets/opt-a.png" || q.Options[1].ImgSrc != "" {
		t.Fatalf("imgSrc not decoded: %+v", q.Options)
	}
	if len(q.CorrectAnswer.Options) != 1 || q.CorrectAnswer.Options[0].ImgSrc != "assets/opt-a.png" {
		t.Fatalf("imgSrc lost from correct answer: %+v", q.CorrectAnswer.Options)
	}

	out, err := xml.Marshal(q)
	if err != nil {
		t.Fatalf("Marshal: unexpected error: %v", err)
	}
	if !strings.Contains(string(out), `<option id="1" imgSrc="assets/opt-a.png"></option>`) {
		t.Fatalf("imgSrc missing from marshaled XML: %s", out)
	}
	if strings.Contains(string(out), `<option id="2" imgSrc=`) {
		t.Fatalf("empty imgSrc emitted for text-only option: %s", out)
	}
}

// virtualCollectionExamXML builds an exam document of the given category,
// carrying two question collections (2 and 1 questions) plus the given
// <virtualcollection> block, for virtual-collection validation tests.
func virtualCollectionExamXML(category, vcBlock string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<root>
<exam id="1" shortname="X" code="1">
  <title>t</title><description>d</description>
  <examcategory>` + category + `</examcategory>
  ` + vcBlock + `
  <questionset>
    <questioncollection>
      <question id="1" type="single-choice"><description>a</description></question>
      <question id="2" type="single-choice"><description>b</description></question>
    </questioncollection>
    <questioncollection>
      <question id="3" type="multiple-choice"><description>c</description></question>
    </questioncollection>
  </questionset>
</exam>
</root>`
}

func TestExamLoader_LoadVirtualCollection(t *testing.T) {
	xml := virtualCollectionExamXML("certification-exam",
		`<virtualcollection><samplesize>2</samplesize><collectionidx>0</collectionidx><collectionidx>1</collectionidx></virtualcollection>`)

	exam, err := NewFileExamLoader().Load([]byte(xml))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	vc := exam.VirtualCollection
	if vc == nil {
		t.Fatal("virtual collection not decoded")
	}
	if vc.SampleSize != 2 || len(vc.CollectionIdx) != 2 || vc.CollectionIdx[0] != 0 || vc.CollectionIdx[1] != 1 {
		t.Fatalf("unexpected virtual collection: %+v", vc)
	}
}

func TestExamLoader_LoadRejectsInvalidVirtualCollection(t *testing.T) {
	for _, tc := range []struct {
		name     string
		category string
		vcBlock  string
		wantErr  string
	}{
		{
			name:     "practice exam",
			category: "practice-exam",
			vcBlock:  `<virtualcollection><samplesize>2</samplesize><collectionidx>0</collectionidx></virtualcollection>`,
			wantErr:  "only allowed in a certification exam",
		},
		{
			name:     "non-positive sample size",
			category: "certification-exam",
			vcBlock:  `<virtualcollection><samplesize>0</samplesize><collectionidx>0</collectionidx></virtualcollection>`,
			wantErr:  "must be positive",
		},
		{
			name:     "index out of range",
			category: "certification-exam",
			vcBlock:  `<virtualcollection><samplesize>1</samplesize><collectionidx>2</collectionidx></virtualcollection>`,
			wantErr:  "unknown question collection index 2",
		},
		{
			name:     "negative index",
			category: "certification-exam",
			vcBlock:  `<virtualcollection><samplesize>1</samplesize><collectionidx>-1</collectionidx></virtualcollection>`,
			wantErr:  "unknown question collection index -1",
		},
		{
			name:     "duplicate index",
			category: "certification-exam",
			vcBlock:  `<virtualcollection><samplesize>2</samplesize><collectionidx>0</collectionidx><collectionidx>0</collectionidx></virtualcollection>`,
			wantErr:  "index 0 twice",
		},
		{
			name:     "population smaller than sample size",
			category: "certification-exam",
			vcBlock:  `<virtualcollection><samplesize>4</samplesize><collectionidx>0</collectionidx><collectionidx>1</collectionidx></virtualcollection>`,
			wantErr:  "exceeds the 3 questions available",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			xml := virtualCollectionExamXML(tc.category, tc.vcBlock)
			_, err := NewFileExamLoader().Load([]byte(xml))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Load: error = %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestExamLoader_LoadTotalExamScore(t *testing.T) {
const xml = `<?xml version="1.0" encoding="UTF-8"?>
<root>
<exam id="1" shortname="X" code="1">
  <title>t</title><description>d</description>
  <passingscore>70</passingscore>
  <totalExamScore>120</totalExamScore>
  <examcategory>certification-exam</examcategory>
  <virtualcollection><samplesize>1</samplesize><collectionidx>0</collectionidx></virtualcollection>
  <questionset>
    <questioncollection>
      <question id="1" type="single-choice"><description>a</description></question>
    </questioncollection>
  </questionset>
</exam>
</root>`

exam, err := NewFileExamLoader().Load([]byte(xml))
if err != nil {
	t.Fatalf("Load: unexpected error: %v", err)
}
if exam.TotalExamScore == nil || *exam.TotalExamScore != 120 {
	t.Fatalf("TotalExamScore = %v, want 120", exam.TotalExamScore)
}
}

func TestExamLoader_LoadRejectsTotalExamScoreInPracticeExam(t *testing.T) {
const xml = `<?xml version="1.0" encoding="UTF-8"?>
<root>
<exam id="1" shortname="X" code="1">
  <title>t</title><description>d</description>
  <totalExamScore>120</totalExamScore>
  <examcategory>practice-exam</examcategory>
  <questionset><questioncollection></questioncollection></questionset>
</exam>
</root>`

_, err := NewFileExamLoader().Load([]byte(xml))
if err == nil || !strings.Contains(err.Error(), "only allowed in a certification exam") {
	t.Fatalf("Load: error = %v, want one containing %q", err, "only allowed in a certification exam")
}
}

func TestExamExcerptFrom(t *testing.T) {
total := float32(120)
coll := QuestionCollection{Questions: []Question{
	{Id: "q1", Type: QuestionTypeSingleChoice, Score: 1},
	{Id: "q2", Type: QuestionTypeSingleChoice, Score: 2},
}}
for _, tc := range []struct {
	name      string
	exam      *Exam
	wantNum   int
	wantTotal float32
}{
	{
		name: "virtual collection reports sample size and total exam score",
		exam: &Exam{
			Id:                "1",
			ExamCategory:      ExamCategoryCertification,
			TotalExamScore:    &total,
			VirtualCollection: &VirtualCollection{SampleSize: 5, CollectionIdx: []int{0}},
			QuestionSet:       QuestionSet{QuestionCollections: []QuestionCollection{coll}},
		},
		wantNum:   5,
		wantTotal: 120,
	},
	{
		name: "virtual collection without total exam score scores zero",
		exam: &Exam{
			Id:                "1",
			ExamCategory:      ExamCategoryCertification,
			VirtualCollection: &VirtualCollection{SampleSize: 5, CollectionIdx: []int{0}},
			QuestionSet:       QuestionSet{QuestionCollections: []QuestionCollection{coll}},
		},
		wantNum:   5,
		wantTotal: 0,
	},
	{
		name: "no virtual collection sums the first collection",
		exam: &Exam{
			Id:           "1",
			ExamCategory: ExamCategoryPractice,
			QuestionSet:  QuestionSet{QuestionCollections: []QuestionCollection{coll}},
		},
		wantNum:   2,
		wantTotal: 3,
	},
	{
		name: "no question collections reports zeros",
		exam: &Exam{
			Id:           "1",
			ExamCategory: ExamCategoryPractice,
		},
		wantNum:   0,
		wantTotal: 0,
	},
} {
	t.Run(tc.name, func(t *testing.T) {
		excerpt := ExamExcerptFrom(tc.exam)
		if excerpt.NumQuestions != tc.wantNum || excerpt.TotalScores != tc.wantTotal {
			t.Fatalf("excerpt = %+v, want NumQuestions %d and TotalScores %g", excerpt, tc.wantNum, tc.wantTotal)
		}
	})
}
}
