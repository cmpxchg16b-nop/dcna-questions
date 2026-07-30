package main

import (
	"encoding/json"
	"fmt"
	"log"

	"dcna-questions/pkg/models/question"
)

func main() {
	exam, err := question.NewExamLoader().LoadFile("exam1.xml")
	if err != nil {
		log.Fatal(err)
	}

	out, err := json.MarshalIndent(exam, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(out))
}
