// Package question defines the data model for the question bank.
package question

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type QuestionType string

const (
	QuestionTypeSingleChoice   QuestionType = "single-choice"
	QuestionTypeMultipleChoice QuestionType = "multiple-choice"
	QuestionTypeDragAndDrop    QuestionType = "drag-and-drop"
)

// Valid reports whether t is one of the recognized question types.
func (t QuestionType) Valid() bool {
	switch t {
	case QuestionTypeSingleChoice, QuestionTypeMultipleChoice, QuestionTypeDragAndDrop:
		return true
	}
	return false
}

// PlainText is a text node in the question document.
type PlainText string

type Option struct {
	Id      string    `xml:"id,attr" json:"id"`
	Content PlainText `xml:",chardata" json:"content"`
}

type Options []Option

type Candidate struct {
	Id      string    `xml:"id,attr" json:"id"`
	Content PlainText `xml:",chardata" json:"content"`
}

type Candidates []Candidate

type Drop struct {
	Id      string    `xml:"id,attr" json:"id"`
	Content PlainText `xml:",chardata" json:"content"`
}

type Drops []Drop

// DropArea is one labeled sub-section of a multi-area drop zone. It carries its
// own set of drop targets, which share the question-wide drop id namespace.
type DropArea struct {
	Id    string    `xml:"id,attr" json:"id"`
	Label PlainText `xml:"droparealabel" json:"label,omitempty"`
	Drops Drops     `xml:"drop" json:"drops"`
}

// MultiAreaDrop models a <multiareadrop>: a drag-and-drop drop area split into
// one or more labeled sub-sections, in contrast to the flat <drops> list.
type MultiAreaDrop struct {
	DropAreas []DropArea `xml:"droparea" json:"dropAreas"`
}

// ImgCandidate is an <imgCandidate>: a draggable image snippet identified by
// its nodeId. NodeLabel is the descriptive text used when rendering the
// correct answer in the frontend. Width and Height give the intrinsic pixel
// size of the snippet.
type ImgCandidate struct {
	NodeId     string `xml:"nodeId,attr" json:"nodeId"`
	NodeLabel  string `xml:"nodeLabel,attr" json:"nodeLabel"`
	Width      int    `xml:"width,attr" json:"width"`
	Height     int    `xml:"height,attr" json:"height"`
	ImgDataSrc string `xml:"imgDataSrc,attr" json:"imgDataSrc"`
}

// ImgDrop is an <imgDrop>: a single drop target laid out at an absolute
// position over the drop area background image. NodeLabel is the descriptive
// text used when rendering the correct answer in the frontend.
type ImgDrop struct {
	NodeId    string `xml:"nodeId,attr" json:"nodeId"`
	NodeLabel string `xml:"nodeLabel,attr" json:"nodeLabel"`
	PositionX int    `xml:"positionX,attr" json:"positionX"`
	PositionY int    `xml:"positionY,attr" json:"positionY"`
	Width     int    `xml:"width,attr" json:"width"`
	Height    int    `xml:"height,attr" json:"height"`
}

// ImgDropsArea is the single <imgDropsArea> within an <imgDragAndDrop>: a
// background image that hosts one or more absolutely positioned <imgDrop>
// targets.
type ImgDropsArea struct {
	ImgBackgroundUrl string    `xml:"imgBackgroundUrl,attr" json:"imgBackgroundUrl"`
	Width            int       `xml:"width,attr" json:"width"`
	Height           int       `xml:"height,attr" json:"height"`
	ImgDrops         []ImgDrop `xml:"imgDrop" json:"imgDrops"`
}

// ImgDragAndDrop models an <imgDragAndDrop>: an image-based drag-and-drop
// question where the candidate drags image snippets (ImgCandidates) onto
// absolutely positioned drop targets (ImgDrops) over a background image.
type ImgDragAndDrop struct {
	ImgCandidates []ImgCandidate `xml:"imgCandidate" json:"imgCandidates"`
	ImgDropsArea  ImgDropsArea   `xml:"imgDropsArea" json:"imgDropsArea"`
}

type Image struct {
	Src string `xml:"src,attr" json:"src"`
}

type Exhibit struct {
	Image Image `xml:"image" json:"image"`
}

type Exhibits []Exhibit

type Connect struct {
	Src string `xml:"src,attr" json:"src"`
	Dst string `xml:"dst,attr" json:"dst"`
}

type ConnectSource struct {
	Id string `xml:"id,attr" json:"id"`
}

type ConnectDestination struct {
	Id string `xml:"id,attr" json:"id"`
}

// ConnectCombination models a <connectcombination>: the Cartesian product of its
// ConnectSources and ConnectDestinations yields the set of connections that are
// considered correct.
type ConnectCombination struct {
	ConnectSources      []ConnectSource      `xml:"connectsource" json:"connectSources,omitempty"`
	ConnectDestinations []ConnectDestination `xml:"connectdestination" json:"connectDestinations,omitempty"`
}

// ConnectionSolution models a <connectionsolution>. Its correctness is
// determined by meeting requiredUniqueConnections unique connections, drawn from
// either explicit Connects or the products of ConnectCombinations.
type ConnectionSolution struct {
	RequiredUniqueConnections int                  `xml:"requiredUniqueConnections,attr" json:"requiredUniqueConnections"`
	Connects                  []Connect            `xml:"connect" json:"connects,omitempty"`
	ConnectCombinations       []ConnectCombination `xml:"connectcombination" json:"connectCombinations,omitempty"`
}

type Combination struct {
	Options Options `xml:"option" json:"options"`
}

// CorrectAnswer is polymorphic: a question's answer is exactly one of an
// option set (single-choice), one or more combinations (multiple-choice), or
// one or more connection solutions (drag-and-drop).
type CorrectAnswer struct {
	Options             Options              `xml:"options>option" json:"options,omitempty"`
	Combinations        []Combination        `xml:"combination" json:"combinations,omitempty"`
	ConnectionSolutions []ConnectionSolution `xml:"connectionsolutions>connectionsolution" json:"connectionSolutions,omitempty"`
}

type QuestionDescription struct {
	Text PlainText `xml:",chardata" json:"text"`
}

type Question struct {
	Id             string              `xml:"id,attr" json:"id"`
	Type           QuestionType        `xml:"type,attr" json:"type"`
	Score          float32             `xml:"score,attr" json:"score,omitempty"`
	Description    QuestionDescription `xml:"description" json:"description"`
	Exhibits       Exhibits            `xml:"exhibits>exhibit" json:"exhibits,omitempty"`
	Options        Options             `xml:"options>option" json:"options,omitempty"`
	Candidates     Candidates          `xml:"candidates>candidate" json:"candidates,omitempty"`
	ImgDragAndDrop *ImgDragAndDrop     `xml:"imgDragAndDrop" json:"imgDragAndDrop,omitempty"`
	MultiAreaDrop  *MultiAreaDrop      `xml:"multiareadrop" json:"multiAreaDrop,omitempty"`
	Drops          Drops               `xml:"drops>drop" json:"drops,omitempty"`
	CorrectAnswer  CorrectAnswer       `xml:"correctanswer" json:"correctAnswer"`
}

// QuestionCollection is a named group of questions. It models a single
// <questioncollection> element, which contains zero or more <question> children.
type QuestionCollection struct {
	Questions []Question `xml:"question" json:"questions,omitempty"`
}

// QuestionSet is the single <questionset> within an exam. It groups zero, one or
// more question collections; a subset may be chosen at random to vary the exam.
type QuestionSet struct {
	QuestionCollections []QuestionCollection `xml:"questioncollection" json:"questionCollections,omitempty"`
}

// Exam is the root <exam> document: a named certification exam carrying
// metadata and exactly one question set.
type Exam struct {
	XMLName      xml.Name     `xml:"exam" json:"-"`
	Id           string       `xml:"id,attr" json:"id"`
	ShortName    string       `xml:"shortname,attr" json:"shortName"`
	Code         string       `xml:"code,attr" json:"code"`
	Title        PlainText    `xml:"title" json:"title"`
	Description  PlainText    `xml:"description" json:"description"`
	PassingScore *float32     `xml:"passingscore" json:"passingScore,omitempty"`
	ExamCategory ExamCategory `xml:"examcategory" json:"examCategory"`
	QuestionSet  QuestionSet  `xml:"questionset" json:"questionSet"`
}

// ExamCategory is the value of the <examcategory> element in an exam. It mirrors
// the XSD ExamCategory enumeration, classifying an exam as a proctored
// certification exam or an unproctored practice exam.
type ExamCategory string

const (
	ExamCategoryCertification ExamCategory = "certification-exam"
	ExamCategoryPractice      ExamCategory = "practice-exam"
)

// Valid reports whether c is one of the recognized exam categories.
func (c ExamCategory) Valid() bool {
	switch c {
	case ExamCategoryCertification, ExamCategoryPractice:
		return true
	}
	return false
}

// OverallResult is the value of the <overallresult> element in an assessment.
// It mirrors the XSD OverallResultType enumeration.
type OverallResult string

const (
	OverallResultPass      OverallResult = "pass"
	OverallResultImmediate OverallResult = "immediate"
)

// Valid reports whether r is one of the recognized overall results.
func (r OverallResult) Valid() bool {
	switch r {
	case OverallResultPass, OverallResultImmediate:
		return true
	}
	return false
}

// ScoreResult is the <scoreresult> element: the aggregate earned and total
// score for a graded submission.
type ScoreResult struct {
	EarnedScore float32 `xml:"earnedScore,attr" json:"earnedScore"`
	TotalScore  float32 `xml:"totalScore,attr" json:"totalScore"`
}

// QuestionScore is one <questionscore> element: the score earned on a single
// question, identified by its questionId.
type QuestionScore struct {
	QuestionId  string  `xml:"questionId,attr" json:"questionId"`
	ScoreEarned float32 `xml:"scoreEarned,attr" json:"scoreEarned"`
}

// Assessment is the <assessment> element: the score report produced after a
// submission is graded. It carries the overall result, the aggregate score,
// and a per-question score breakdown. All children are optional (0..1 / 0..n).
//
// Questions holds the original <question> documents, but only for
// practice-exam category submissions: the XSD documents that the origin
// question (and therefore its correct answer) is included so the candidate can
// review what they got wrong, whereas certification-exam omits it. The grader
// populates Questions only for the questions the candidate actually answered.
type Assessment struct {
	XMLName        xml.Name        `xml:"assessment" json:"-"`
	OverallResult  *OverallResult  `xml:"overallresult" json:"overallResult,omitempty"`
	ScoreResult    *ScoreResult    `xml:"scoreresult" json:"scoreResult,omitempty"`
	QuestionScores []QuestionScore `xml:"questionscore" json:"questionScores,omitempty"`
	Questions      []Question      `xml:"question" json:"questions,omitempty"`
}

// Answer is one <answer> element within an examanswer: a candidate's response
// to a single question. For single-choice and multiple-choice questions it
// carries the selected options; for drag-and-drop questions it carries the
// connections. QuestionType is recorded on the answer so a submission can be
// validated against the schema without re-reading the question.
type Answer struct {
	QuestionId   string       `xml:"questionId,attr" json:"questionId"`
	QuestionType QuestionType `xml:"questionType,attr" json:"questionType"`
	Options      Options      `xml:"options>option" json:"options,omitempty"`
	Connections  []Connect    `xml:"connections>connect" json:"connections,omitempty"`
}

// ExamAnswer is the <examanswer> element: a complete submission carrying one
// answer per answered question for the chosen question collection.
type ExamAnswer struct {
	XMLName xml.Name `xml:"examanswer" json:"-"`
	Answers []Answer `xml:"answer" json:"answers,omitempty"`
}

// ExamDocumentExcerpt is a lightweight projection of an Exam that carries its
// identifying metadata plus aggregate counts derived from the first question
// collection. It has no behavior: it exists to expose a small, stable summary
// of an exam without handing consumers the full document graph.
type ExamDocumentExcerpt struct {
	// Id of the exam document, not the id of the exam session
	Id string

	ShortName    string
	Code         string
	Title        PlainText
	Description  PlainText
	ExamCategory ExamCategory
	NumQuestions int
	TotalScores  float32
}

// ExamExcerptFrom builds an ExamExcerpt from an Exam. NumQuestions is the
// number of questions in the first question collection and TotalScores is the
// sum of their scores; both are zero when the exam has no question collections.
func ExamExcerptFrom(e *Exam) ExamDocumentExcerpt {
	excerpt := ExamDocumentExcerpt{
		Id:           e.Id,
		ShortName:    e.ShortName,
		Code:         e.Code,
		Title:        e.Title,
		Description:  e.Description,
		ExamCategory: e.ExamCategory,
	}
	if len(e.QuestionSet.QuestionCollections) == 0 {
		return excerpt
	}
	qc := e.QuestionSet.QuestionCollections[0]
	excerpt.NumQuestions = len(qc.Questions)
	for _, q := range qc.Questions {
		excerpt.TotalScores += q.Score
	}
	return excerpt
}

// namedEntities extends the predefined XML entities (amp, lt, gt, apos, quot,
// which encoding/xml resolves by itself) with the full HTML named-entity table.
//
// Numeric character references such as &#8226; (the bullet, the only reference
// form currently used in the question bank) are resolved by encoding/xml
// unconditionally and need no entry here. This table exists solely so that
// named HTML entities (&nbsp;, &copy;, ...) decode correctly should a future
// question bank introduce them. It is built once and shared; encoding/xml only
// reads it, never writes.
var namedEntities = func() map[string]string {
	m := make(map[string]string, len(xml.HTMLEntity))
	for k, v := range xml.HTMLEntity {
		m[k] = v
	}
	return m
}()

type ExamLoader interface {
	LoadFrom(url string) (*Exam, error)
}

// FileExamLoader decodes Exam documents from XML. It is stateless and safe for
// concurrent use; the zero value is ready to use.
type FileExamLoader struct{}

// NewFileExamLoader returns an ExamLoader. It is retained for call-site
// readability; ExamLoader{} is equivalent.
func NewFileExamLoader() *FileExamLoader { return &FileExamLoader{} }

// Load decodes data into an Exam and validates its structure.
//
// The document root is <root>, which wraps exactly one <exam> plus an optional
// <examanswer> holding example answers. Only the <exam> is loaded; the example
// answer is ignored.
func (l *FileExamLoader) Load(data []byte) (*Exam, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Entity = namedEntities
	var doc struct {
		XMLName xml.Name `xml:"root"`
		Exam    Exam     `xml:"exam"`
	}
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode exam XML: %w", err)
	}
	if err := doc.Exam.validate(); err != nil {
		return nil, fmt.Errorf("invalid exam: %w", err)
	}
	return &doc.Exam, nil
}

// LoadFile reads the XML file at path and decodes it into an Exam.
func (l *FileExamLoader) LoadFile(path string) (*Exam, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read exam file %q: %w", path, err)
	}
	return l.Load(data)
}

// LoadFile reads the XML file at path and decodes it into an Exam.
func (l *FileExamLoader) LoadFrom(url string) (*Exam, error) {
	return l.LoadFile(url)
}

// validate reports structural problems with a decoded exam. It catches the
// kinds of mistakes (a typo'd question type, a missing id) that would otherwise
// produce silently broken questions downstream.
func (e *Exam) validate() error {
	if e.Id == "" {
		return errors.New("missing exam id")
	}
	if !e.ExamCategory.Valid() {
		return fmt.Errorf("exam %q: unknown exam category %q", e.Id, e.ExamCategory)
	}
	for _, qc := range e.QuestionSet.QuestionCollections {
		for _, q := range qc.Questions {
			if q.Id == "" {
				return errors.New("question with missing id")
			}
			if !q.Type.Valid() {
				return fmt.Errorf("question %q: unknown type %q", q.Id, q.Type)
			}
		}
	}
	return nil
}

// ExamSourceEntry describes one source of exam documents: a Loader (an abstract type
// that knows how to decode an Exam from a URL) plus the URLs it provides.
type ExamSourceEntry struct {
	Loader ExamLoader
	URLs   []string
}

type StaticFileExamSource struct {
	entries []ExamSourceEntry
}

func NewStaticFileExamSource(entries []ExamSourceEntry) *StaticFileExamSource {
	return &StaticFileExamSource{entries: entries}
}

func (s *StaticFileExamSource) Get() []ExamSourceEntry {
	return s.entries
}

type DynamicDirExamSource struct {
	dir string
}

func NewDynamicDirExamSource(dir string) *DynamicDirExamSource {
	return &DynamicDirExamSource{dir: dir}
}

func (s *DynamicDirExamSource) Get() []ExamSourceEntry {
	files, err := os.ReadDir(s.dir)
	if err != nil {
		return nil
	}
	var urls []string
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".xml") {
			continue
		}
		urls = append(urls, filepath.Join(s.dir, f.Name()))
	}
	if len(urls) == 0 {
		return nil
	}
	return []ExamSourceEntry{{Loader: NewFileExamLoader(), URLs: urls}}
}

type ExamSource interface {
	Get() []ExamSourceEntry
}

// ExamRepository aggregates exam documents drawn from one or more ExamSources.
type ExamRepository struct {
	sources []ExamSource

	// cache holds exams keyed by id (id -> *Exam) so lookups by id avoid
	// re-reading from disk. It is a sync.Map because it is written
	// incrementally as exams are loaded — by ListExamDocuments, which inserts
	// each exam as it streams it, and by reload, which GetExamDocumentById runs
	// on a miss — while GetExamDocumentById reads it concurrently.
	cache sync.Map
}

// NewExamRepository constructs an ExamRepository over the given sources.
func NewExamRepository(sources []ExamSource) *ExamRepository {
	return &ExamRepository{sources: sources}
}

// cacheExam stores exam in the cache keyed by its id.
func (r *ExamRepository) cacheExam(exam *Exam) {
	r.cache.Store(exam.Id, exam)
}

// cachedExam returns the cached exam for id, or (nil, false) if none is cached.
func (r *ExamRepository) cachedExam(id string) (*Exam, bool) {
	v, ok := r.cache.Load(id)
	if !ok {
		return nil, false
	}
	return v.(*Exam), true
}

// reload reads every exam from every source and caches it by id. It does not
// evict ids that were previously cached but are no longer present in the
// sources. A source URL that fails to load is skipped: it contributes no id to
// the cache, so a later lookup for an exam that failed to load reports "not
// found" rather than its underlying load error.
func (r *ExamRepository) reload() {
	for _, src := range r.sources {
		for _, entry := range src.Get() {
			for _, url := range entry.URLs {
				exam, err := entry.Loader.LoadFrom(url)
				if err != nil {
					continue
				}
				r.cacheExam(exam)
			}
		}
	}
}

// ExamDataEvent is one item emitted by ListExamDocuments: a successfully loaded
// exam (Data) or, when loading a URL failed, the error (Err). Exactly one of
// Err or Data is non-nil per event; the consumer distinguishes them by checking
// the Err field.
type ExamDataEvent struct {
	Err  error
	Data *Exam
}

// ListExamDocuments streams every exam from every source over a single
// unbuffered channel of ExamDataEvent, multiplexing successes and failures so
// the caller can consume them with a single range. Each exam is loaded and
// emitted one at a time; loading is lazy (nothing is materialized before it is
// streamed), and each successfully loaded exam is also inserted into the cache,
// so the disk read that powers the stream also feeds GetExamDocumentById. On
// error the error is emitted as an event and loading continues; the
// callee-spawned goroutine closes the channel once every source has been
// exhausted. The caller tests for failure by checking the event's Err field.
func (r *ExamRepository) ListExamDocuments() <-chan ExamDataEvent {
	events := make(chan ExamDataEvent)
	go func() {
		defer close(events)
		for _, src := range r.sources {
			for _, entry := range src.Get() {
				for _, url := range entry.URLs {
					exam, err := entry.Loader.LoadFrom(url)
					if err != nil {
						events <- ExamDataEvent{Err: fmt.Errorf("load exam %q: %w", url, err)}
						continue
					}
					r.cacheExam(exam) // refresh the cache as each exam loads
					events <- ExamDataEvent{Data: exam}
				}
			}
		}
	}()
	return events
}

// GetExamDocumentById returns the exam whose id matches. It consults the cache
// first; only on a miss does it reload the entire collection and re-check. The
// cache is also populated as a side effect of ListExamDocuments, so lookups for
// already-streamed exams hit without touching the sources. A lookup that still
// misses after a reload returns "not found".
func (r *ExamRepository) GetExamDocumentById(id string) (*Exam, error) {
	if exam, ok := r.cachedExam(id); ok {
		return exam, nil
	}
	r.reload()
	if exam, ok := r.cachedExam(id); ok {
		return exam, nil
	}
	return nil, fmt.Errorf("exam %q not found", id)
}
