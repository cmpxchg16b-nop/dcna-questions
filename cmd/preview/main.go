package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"os"

	"dcna-questions/pkg/models/question"
)

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
		Questions question.Questions `xml:"question"`
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
