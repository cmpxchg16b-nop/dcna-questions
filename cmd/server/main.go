package main

import (
	dcnaquestions "dcna-questions"
	pkgapicounter "dcna-questions/pkg/api/counter"
	pkgapiexamdocs "dcna-questions/pkg/api/examdocs"
	pkgapiexamsessions "dcna-questions/pkg/api/examsessions"
	pkgmodelsexamserver "dcna-questions/pkg/models/examserver"
	pkgmodelsquestion "dcna-questions/pkg/models/question"
	pkgsession "dcna-questions/pkg/session"

	"context"
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
	examHandler := pkgapiexamdocs.NewExamHandler(repo)

	examServer := pkgmodelsexamserver.NewOnMemoryExamServer()
	go examServer.Run(context.Background())
	defer examServer.Shutdown()

	sm := pkgsession.NewOnMemorySessionManager()
	examSessionHandler := pkgapiexamsessions.NewExamSessionHandler(sm, examServer, repo)

	mux := http.NewServeMux()
	mux.Handle("/api/counter", pkgapicounter.NewHandler(sm))
	mux.Handle("/api/examdocs", examHandler)
	mux.Handle("/api/examsessions", examSessionHandler)
	mux.Handle("/api/examsessions/", examSessionHandler)
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
