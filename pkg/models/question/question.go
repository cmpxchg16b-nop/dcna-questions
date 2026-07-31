// Package question defines the data model for the question bank.
package question

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
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
	Id            string              `xml:"id,attr" json:"id"`
	Type          QuestionType        `xml:"type,attr" json:"type"`
	Score         int                 `xml:"score,attr" json:"score,omitempty"`
	Description   QuestionDescription `xml:"description" json:"description"`
	Exhibits      Exhibits            `xml:"exhibits>exhibit" json:"exhibits,omitempty"`
	Options       Options             `xml:"options>option" json:"options,omitempty"`
	Candidates    Candidates          `xml:"candidates>candidate" json:"candidates,omitempty"`
	MultiAreaDrop *MultiAreaDrop      `xml:"multiareadrop" json:"multiAreaDrop,omitempty"`
	Drops         Drops               `xml:"drops>drop" json:"drops,omitempty"`
	CorrectAnswer CorrectAnswer       `xml:"correctanswer" json:"correctAnswer"`
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
	XMLName     xml.Name    `xml:"exam" json:"-"`
	Id          string      `xml:"id,attr" json:"id"`
	ShortName   string      `xml:"shortname,attr" json:"shortName"`
	Code        string      `xml:"code,attr" json:"code"`
	Title       PlainText   `xml:"title" json:"title"`
	Description PlainText   `xml:"description" json:"description"`
	QuestionSet QuestionSet `xml:"questionset" json:"questionSet"`
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
