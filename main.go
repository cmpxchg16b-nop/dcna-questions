package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"os"
)

type QuestionType string

const (
	QuestionTypeSingleChoice   QuestionType = "single-choice"
	QuestionTypeMultipleChoice QuestionType = "multiple-choice"
	QuestionTypeDragAndDrop    QuestionType = "drag-and-drop"
)

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

type ConnectionSolution struct {
	Connects []Connect `xml:"connect" json:"connects"`
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
	Num           int                 `xml:"num,attr" json:"num"`
	Type          QuestionType        `xml:"type,attr" json:"type"`
	Description   QuestionDescription `xml:"description" json:"description"`
	Exhibits      Exhibits            `xml:"exhibits>exhibit" json:"exhibits,omitempty"`
	Options       Options             `xml:"options>option" json:"options,omitempty"`
	Candidates    Candidates          `xml:"candidates>candidate" json:"candidates,omitempty"`
	Drops         Drops               `xml:"drops>drop" json:"drops,omitempty"`
	CorrectAnswer CorrectAnswer       `xml:"correctanswer" json:"correctAnswer"`
}

type Questions []Question

// entityMap returns the set of XML character entities the decoder should
// resolve. It extends the standard HTML entity set with a few extra symbols
// (e.g. &bullet;) used in the question bank.
func entityMap() map[string]string {
	m := make(map[string]string, len(xml.HTMLEntity)+1)
	for k, v := range xml.HTMLEntity {
		m[k] = v
	}
	m["bullet"] = "\u2022"
	return m
}

func main() {
	data, err := os.ReadFile("questions.xml")
	if err != nil {
		log.Fatal(err)
	}

	var doc struct {
		Questions Questions `xml:"question"`
	}

	// Use a Decoder so HTML-style entities such as &bullet; resolve correctly.
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Entity = entityMap()
	if err := dec.Decode(&doc); err != nil {
		log.Fatal(err)
	}

	out, err := json.MarshalIndent(doc.Questions, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(out))
}
