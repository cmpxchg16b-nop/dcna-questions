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
