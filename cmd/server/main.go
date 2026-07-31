package main

import (
	dcnaquestions "dcna-questions"
	pkgapicounter "dcna-questions/pkg/api/counter"
	"dcna-questions/pkg/session"

	pkglog "dcna-questions/pkg/log"
	"log"
	"net/http"
)

func main() {
	sm := session.NewOnMemorySessionManager()

	mux := http.NewServeMux()
	mux.Handle("/api/counter", pkgapicounter.NewHandler(sm))
	mux.Handle("/", dcnaquestions.Handler())

	const addr = ":8080"
	log.Printf("listening on http://localhost%s", addr)

	var h http.Handler = mux
	h = pkglog.WithSessionAwaredLog(nil, sm, h)
	h = session.WithSessionId(h, sm)

	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
	}
}
