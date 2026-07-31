package main

import (
	dcnaquestions "dcna-questions"
	pkgapicounter "dcna-questions/pkg/api/counter"
	pkgapiexam "dcna-questions/pkg/api/exam"
	pkgmodelsquestion "dcna-questions/pkg/models/question"
	pkgsession "dcna-questions/pkg/session"

	pkglog "dcna-questions/pkg/log"
	"log"
	"net/http"
)

func main() {
	sources := []pkgmodelsquestion.ExamSource{
		{
			Loader: pkgmodelsquestion.NewFileExamLoader(),
			URLs:   []string{"exam1.xml"},
		},
	}
	repo := pkgmodelsquestion.NewExamRepository(sources)
	examHandler := pkgapiexam.NewExamHandler(repo)

	sm := pkgsession.NewOnMemorySessionManager()

	mux := http.NewServeMux()
	mux.Handle("/api/counter", pkgapicounter.NewHandler(sm))
	mux.Handle("/api/examdocs", examHandler)
	mux.Handle("/", dcnaquestions.Handler())

	const addr = ":8080"
	log.Printf("listening on http://localhost%s", addr)

	var h http.Handler = mux
	h = pkglog.WithSessionAwaredLog(nil, sm, h)
	h = pkgsession.WithSessionId(h, sm)

	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
	}
}
