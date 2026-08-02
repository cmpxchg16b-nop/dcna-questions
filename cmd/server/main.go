package main

import (
	dcnaquestions "dcna-questions"
	pkgapiexamdocs "dcna-questions/pkg/api/examdocs"
	pkgapiexamsessions "dcna-questions/pkg/api/examsessions"
	pkgmodelsexamserver "dcna-questions/pkg/models/examserver"
	pkgmodelsquestion "dcna-questions/pkg/models/question"
	pkgsession "dcna-questions/pkg/session"

	"context"
	pkglog "dcna-questions/pkg/log"
	"log"
	"net/http"

	"github.com/alecthomas/kong"
)

type CLI struct {
	Addr      string `name:"addr" help:"Listening address." env:"ADDR" default:":8080"`
	AssetsDir string `name:"assets-dir" help:"Directory of static assets to serve under /assets/." env:"ASSETS_DIR" type:"existingdir"`
}

func (cli *CLI) Run() error {
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
	mux.Handle("/api/examdocs", examHandler)
	mux.Handle("/api/examsessions", examSessionHandler)
	mux.Handle("/api/examsessions/", examSessionHandler)

	if cli.AssetsDir != "" {
		mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(cli.AssetsDir))))
	}

	mux.Handle("/", dcnaquestions.Handler())

	log.Printf("listening on http://localhost%s", cli.Addr)

	var h http.Handler = mux
	h = pkglog.WithSessionAwaredLog(nil, sm, h)
	h = pkgsession.WithSessionId(h, sm)

	return http.ListenAndServe(cli.Addr, h)
}

func main() {
	var cli CLI
	ctx := kong.Parse(&cli)
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
