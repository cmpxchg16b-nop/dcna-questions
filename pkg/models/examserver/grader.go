package examserver

import (
	pkgmodelquestions "dcna-questions/pkg/models/question"
)

// SimpleGrader grades an exam submission and returns an assessment of the
// results.
type SimpleGrader interface {
	// Grade evaluates the given examAnswer and returns an Assessment describing
	// the per-question and overall scores, or an error if grading could not be
	// completed.
	Grade(examAnswer *pkgmodelquestions.ExamAnswer) (*pkgmodelquestions.Assessment, error)
}

// SimpleOnMemoryGrader is a SimpleGrader that indexes a question collection in
// memory by question id. It is not safe for concurrent use.
//
// Only single-choice and multiple-choice questions are graded; questions of any
// other type are silently skipped. Answers are correlated to questions by
// questionId, and option sets are compared by option id, so neither question
// nor option ordering affects the result.
type SimpleOnMemoryGrader struct {
	questions    map[string]pkgmodelquestions.Question
	totalScore   float32
	passingScore *float32
}

// NewSimpleOnMemoryGrader indexes qc by question id and precomputes the total
// achievable score. passingScore, when non-nil, drives the assessment's
// OverallResult: the submission passes once the earned score reaches it.
func NewSimpleOnMemoryGrader(qc *pkgmodelquestions.QuestionCollection, passingScore *float32) *SimpleOnMemoryGrader {
	g := &SimpleOnMemoryGrader{
		questions:    make(map[string]pkgmodelquestions.Question),
		passingScore: passingScore,
	}
	if qc != nil {
		for _, q := range qc.Questions {
			g.questions[q.Id] = q
			g.totalScore += q.Score
		}
	}
	return g
}

// Grade evaluates each submitted answer against its official question and
// builds an Assessment. Questions whose type is neither single-choice nor
// multiple-choice, and answers with no matching question, are skipped without
// error. Unanswered questions contribute no QuestionScore; their potential
// score still counts toward TotalScore.
func (g *SimpleOnMemoryGrader) Grade(examAnswer *pkgmodelquestions.ExamAnswer) (*pkgmodelquestions.Assessment, error) {
	var earnedScore float32
	var questionScores []pkgmodelquestions.QuestionScore

	if examAnswer != nil {
		for _, ans := range examAnswer.Answers {
			q, ok := g.questions[ans.QuestionId]
			if !ok {
				continue
			}
			switch q.Type {
			case pkgmodelquestions.QuestionTypeSingleChoice,
				pkgmodelquestions.QuestionTypeMultipleChoice:
			default:
				continue // unsupported type: skip silently
			}

			var earned float32
			if isAnswerCorrect(q, ans) {
				earned = q.Score
			}
			earnedScore += earned
			questionScores = append(questionScores, pkgmodelquestions.QuestionScore{
				QuestionId:  q.Id,
				ScoreEarned: earned,
			})
		}
	}

	overall := pkgmodelquestions.OverallResultImmediate
	if g.passingScore != nil && earnedScore >= *g.passingScore {
		overall = pkgmodelquestions.OverallResultPass
	}

	return &pkgmodelquestions.Assessment{
		OverallResult: &overall,
		ScoreResult: &pkgmodelquestions.ScoreResult{
			EarnedScore: earnedScore,
			TotalScore:  g.totalScore,
		},
		QuestionScores: questionScores,
	}, nil
}

// isAnswerCorrect reports whether ans matches the official correct answer
// for q. Both question types compare option ids against q.CorrectAnswer.Options,
// so option ordering is irrelevant; they differ in match semantics:
//   - single-choice: correct so long as exactly one option is submitted and
//     that option is one of the correct options.
//   - multiple-choice: correct only when the submitted option set exactly
//     matches the correct option set.
func isAnswerCorrect(q pkgmodelquestions.Question, ans pkgmodelquestions.Answer) bool {
	correct, submitted := q.CorrectAnswer.Options, ans.Options
	switch q.Type {
	case pkgmodelquestions.QuestionTypeSingleChoice:
		if len(submitted) != 1 {
			return false
		}
		correctIds := make(map[string]struct{}, len(correct))
		for _, o := range correct {
			correctIds[o.Id] = struct{}{}
		}
		_, ok := correctIds[submitted[0].Id]
		return ok
	case pkgmodelquestions.QuestionTypeMultipleChoice:
		if len(correct) != len(submitted) {
			return false
		}
		seen := make(map[string]struct{}, len(submitted))
		for _, o := range submitted {
			seen[o.Id] = struct{}{}
		}
		for _, o := range correct {
			if _, ok := seen[o.Id]; !ok {
				return false
			}
		}
		return true
	}
	return false
}
