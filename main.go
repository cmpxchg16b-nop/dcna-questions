package main

type QuestionType string

const (
	QuestionTypeSingleChoice QuestionType = "single-choice"
)

type Option struct {
	Id      string
	Label   string
	Content string
}

type Options []Option

type CorrectAnswer struct {
	Opts Options
}

type Question struct {
	Num  int
	Type QuestionType
}

type Questions []Question

func main() {

}
