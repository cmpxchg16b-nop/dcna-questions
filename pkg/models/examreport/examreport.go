// Package examreport defines the data model for an exam report: the full
// report produced after an exam taker finishes an exam session.
//
// It mirrors the <examreport> and <examtaker> elements defined in exam.xsd.
// Assessment-related types (overall result, scores, assessment) are reused from
// the question package.
package examreport

import (
	"encoding/xml"

	"dcna-questions/pkg/models/question"
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

// ExamReport is the <examreport> element: the full report sent to the
// assessment tracking server after an exam taker has finished the exam session.
// The id attribute is the globally unique report id (not the exam document id,
// nor the exam session id).
//
// The exam metadata fields (ExamId, ExamShortName, ExamCode, Title,
// Description, PassingScore, ExamCategory) are copied from the originating exam
// document; ExamSessionId and FinishedAt describe the specific session.
type ExamReport struct {
	XMLName       xml.Name              `xml:"examreport" json:"-"`
	Id            string                `xml:"id,attr" json:"id"`
	ExamTaker     ExamTaker             `xml:"examtaker" json:"examTaker"`
	ExamId        string                `xml:"examid" json:"examId"`
	ExamShortName string                `xml:"examshortname" json:"examShortName,omitempty"`
	ExamCode      string                `xml:"examcode" json:"examCode,omitempty"`
	Title         string                `xml:"title" json:"title"`
	Description   string                `xml:"description" json:"description,omitempty"`
	PassingScore  *float32              `xml:"passingscore" json:"passingScore,omitempty"`
	ExamCategory  question.ExamCategory `xml:"examcategory" json:"examCategory"`
	ExamSessionId string                `xml:"examsessionid" json:"examSessionId"`
	FinishedAt    int64                 `xml:"finishedat" json:"finishedAt"`
	Assessment    question.Assessment   `xml:"assessment" json:"assessment"`
}
