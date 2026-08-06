package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"

	"dcna-questions/pkg/models/question"
)

// rawOpt is a single answer option of a raw NDJSON question.
type rawOpt struct {
	IsCorrect bool   `json:"isCorrect"`
	Text      string `json:"text"`
}

// rawQuestion is one entry in the NDJSON question bank.
// Images is optional in the data; it stays nil when omitted.
type rawQuestion struct {
	Title   string   `json:"title"`
	IsMulti bool     `json:"isMulti"`
	Images  []string `json:"images,omitempty"`
	Opts    []rawOpt `json:"opts"`
}

// Parse reads NDJSON from r and returns one rawQuestion per line.
// Blank lines are skipped; malformed lines report their line number.
func Parse(r io.Reader) ([]rawQuestion, error) {
	var questions []rawQuestion

	scanner := bufio.NewScanner(r)
	// Allow long lines (embedded images, long stems): up to 8 MiB per line.
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var q rawQuestion
		if err := json.Unmarshal(line, &q); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		questions = append(questions, q)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return questions, nil
}

// toModel converts a raw NDJSON question into the exam XML model. id comes
// from the question counter, score is always 1, and the type is derived from
// IsMulti.
func toModel(raw rawQuestion, id int) question.Question {
	typ := question.QuestionTypeSingleChoice
	if raw.IsMulti {
		typ = question.QuestionTypeMultipleChoice
	}

	q := question.Question{
		Id:          strconv.Itoa(id),
		Type:        typ,
		Score:       1,
		Description: question.QuestionDescription{Text: question.PlainText(raw.Title)},
	}

	for _, src := range raw.Images {
		q.Exhibits = append(q.Exhibits, question.Exhibit{Image: question.Image{Src: src}})
	}

	for i, o := range raw.Opts {
		opt := question.Option{Id: strconv.Itoa(i + 1), Content: question.PlainText(o.Text)}
		q.Options = append(q.Options, opt)
		if o.IsCorrect {
			q.CorrectAnswer.Options = append(q.CorrectAnswer.Options, opt)
		}
	}

	return q
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s <questions.ndjson>", os.Args[0])
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	raws, err := Parse(f)
	if err != nil {
		log.Fatal(err)
	}

	enc := xml.NewEncoder(os.Stdout)
	enc.Indent("", "  ")

	counter := 1
	for _, raw := range raws {
		if err := enc.Encode(toModel(raw, counter)); err != nil {
			log.Fatal(err)
		}
		counter++
	}
	if err := enc.Flush(); err != nil {
		log.Fatal(err)
	}
}
