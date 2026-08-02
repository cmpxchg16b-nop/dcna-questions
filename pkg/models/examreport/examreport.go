// Package examreport defines the data model for an exam report: the full
// report produced after an exam taker finishes an exam session.
//
// It mirrors the <examreport> and <examtaker> elements defined in exam.xsd.
// Assessment-related types (overall result, scores, assessment) are reused from
// the question package.
package examreport

import (
	"encoding/xml"

	pkgmodelsquestion "dcna-questions/pkg/models/question"
)

// Person is one named <person> within an <examtaker>: a real exam candidate
// identified by full name. Fistname is spelled as in the XSD attribute.
type Person struct {
	Name     string `xml:"name,attr" json:"name"`
	Fistname string `xml:"fistname,attr" json:"fistname,omitempty"`
	Lastname string `xml:"lastname,attr" json:"lastname,omitempty"`
}

// Anonymous is one <anonymous> entry within an <examtaker>: an unidentified
// exam taker tracked only by session id.
type Anonymous struct {
	SessionId string `xml:"sessionid,attr" json:"sessionId"`
}

// ExamTaker is the <examtaker> element: the list of persons and/or anonymous
// sessions who took the exam. Either may be empty.
type ExamTaker struct {
	XMLName   xml.Name    `xml:"examtaker" json:"-"`
	Persons   []Person    `xml:"person" json:"persons,omitempty"`
	Anonymous []Anonymous `xml:"anonymous" json:"anonymous,omitempty"`
}

// ExamReport is the <examreport> element: a full report sent to the exam
// assessment tracking server after an exam taker has finished the exam session.
type ExamReport struct {
	XMLName xml.Name `xml:"examreport" json:"-"`

	// Id is the id of the exam report; it has to be globally unique, not the id
	// of the exam document, nor the id of the exam session.
	Id string `xml:"id,attr" json:"id"`

	// ExamTaker is the person or anonymous session that took the exam.
	ExamTaker ExamTaker `xml:"examtaker" json:"examTaker"`

	// ExamId is the exam document id, not the exam session id.
	ExamId string `xml:"examid" json:"examId"`

	// ExamShortName is the short name copied from the origin exam document.
	ExamShortName string `xml:"examshortname" json:"examShortName,omitempty"`

	// ExamCode is the code copied from the origin exam document.
	ExamCode string `xml:"examcode" json:"examCode,omitempty"`

	// Title is the title of the exam.
	Title string `xml:"title" json:"title"`

	// Description is the description of the exam. Optional.
	Description string `xml:"description" json:"description,omitempty"`

	// PassingScore is the mandated passing score of the exam, copied directly
	// from the exam element.
	PassingScore *float32 `xml:"passingscore" json:"passingScore,omitempty"`

	// ExamCategory is copied directly from the origin exam document too.
	ExamCategory pkgmodelsquestion.ExamCategory `xml:"examcategory" json:"examCategory"`

	// ExamSessionId is the id of the exam session which the exam taker was in.
	ExamSessionId string `xml:"examsessionid" json:"examSessionId"`

	// FinishedAt is the millisecond-resolution unix timestamp when the exam
	// session was finished by the exam taker.
	FinishedAt int64 `xml:"finishedat" json:"finishedAt"`

	// Assessment contains the grade and the score that was achieved by the
	// exam taker.
	Assessment pkgmodelsquestion.Assessment `xml:"assessment" json:"assessment"`
}
