package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"regexp"
)

type QuestionType string

const (
	QuestionTypeSingleChoice QuestionType = "single-choice"
)

// lineBreakSpace matches a line break followed by any number of spaces.
var lineBreakSpace = regexp.MustCompile(`\r?\n[ ]*`)

// PlainText is a text node whose line breaks are joined on parse: any line
// break followed by spaces collapses into a single space.
type PlainText string

func (p *PlainText) UnmarshalText(text []byte) error {
	*p = PlainText(lineBreakSpace.ReplaceAllString(string(text), " "))
	return nil
}

type Option struct {
	Id      string    `xml:"id,attr" json:"id"`
	Label   string    `xml:"label,attr" json:"label"`
	Content PlainText `xml:",chardata" json:"content"`
}

type Options []Option

type CorrectAnswer struct {
	Opts Options `xml:"options>option" json:"options"`
}

type QuestionDescription struct {
	Text PlainText `xml:",chardata" json:"text"`
}

type Question struct {
	Num           int                 `xml:"num,attr" json:"num"`
	Type          QuestionType        `xml:"type,attr" json:"type"`
	Description   QuestionDescription `xml:"description" json:"description"`
	Options       Options             `xml:"options>option" json:"options"`
	CorrectAnswer CorrectAnswer       `xml:"correctanswer" json:"correctAnswer"`
}

type Questions []Question

func main() {
	data, err := os.ReadFile("questions.xml")
	if err != nil {
		log.Fatal(err)
	}

	var doc struct {
		Questions Questions `xml:"question"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		log.Fatal(err)
	}

	out, err := json.MarshalIndent(doc.Questions, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(out))
}
