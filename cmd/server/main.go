package main

import (
	dcnaquestions "dcna-questions"
	pkgapiexamdocs "dcna-questions/pkg/api/examdocs"
	pkgapiexamsessions "dcna-questions/pkg/api/examsessions"
	pkgapiexamtrackings "dcna-questions/pkg/api/examtrackings"
	pkgauth "dcna-questions/pkg/auth"
	pkgmodelsexamreport "dcna-questions/pkg/models/examreport"
	pkgmodelsexamserver "dcna-questions/pkg/models/examserver"
	pkgmodelsquestion "dcna-questions/pkg/models/question"
	pkgsession "dcna-questions/pkg/session"
	"fmt"
	"os"

	"context"
	pkglog "dcna-questions/pkg/log"
	"errors"
	"log/slog"
	"net/http"

	"github.com/alecthomas/kong"
)

// logger is the application-wide structured logger used by the HTTP logging
// middleware. It defaults to slog.Default() (text handler on stderr).
var logger = slog.Default()

type CLI struct {
	Addr        string   `name:"addr" help:"Listening address." env:"ADDR" default:":8080"`
	AssetsDir   string   `name:"assets-dir" help:"Directory of static assets to serve under /assets/." env:"ASSETS_DIR" type:"existingdir"`
	LoadExam    []string `name:"load-exam" help:"Paths to exam documents to load." env:"LOAD_EXAM" type:"existingfile"`
	LoadExamDir []string `name:"load-exam-dir" help:"Directories of exam documents to load." env:"LOAD_EXAM_DIR" type:"existingdir"`
	JWTAuthSecretFromEnv  string        `name:"jwt-auth-secret-from-env" help:"Name of the environment variable that contains the JWT secret"`
	JWTAuthSecretFromFile string        `name:"jwt-auth-secret-from-file" help:"Path to the file that contains the JWT secret"`
	SubjectBlacklistTxtPath string `name:"subj-blacklist-path" help:"Path to the blacklist text file, one subject id per a line"`
	RejectVisitor         bool          `name:"reject-visitor" help:"Reject requests from visitors (subjects with the 'visitor:' prefix)" default:"false"`
}

func (cli *CLI) Run() error {
	var sources []pkgmodelsquestion.ExamSource

	if len(cli.LoadExam) > 0 {
		sources = append(sources, pkgmodelsquestion.NewStaticFileExamSource([]pkgmodelsquestion.ExamSourceEntry{
			{Loader: pkgmodelsquestion.NewFileExamLoader(), URLs: cli.LoadExam},
		}))
	}

	for _, dir := range cli.LoadExamDir {
		sources = append(sources, pkgmodelsquestion.NewDynamicDirExamSource(dir))
	}

	if len(sources) == 0 {
		return errors.New("no exam sources configured; provide at least one --load-exam or --load-exam-dir")
	}

	repo := pkgmodelsquestion.NewExamRepository(sources)
	examHandler := pkgapiexamdocs.NewExamHandler(repo)

	// A single ExamTrackingServer is shared by the exam server (which persists
	// finished-session reports) and the /api/examtrackings handler (which reads
	// them back), so a report written on session end is immediately visible to
	// the caller.
	trackingServer := pkgmodelsexamreport.NewOnMemoryExamTrackingServer()
	examServer := pkgmodelsexamserver.NewOnMemoryExamServer(trackingServer)
	go examServer.Run(context.Background())
	defer examServer.Shutdown()

	sm := pkgsession.NewOnMemorySessionManager()
	examSessionHandler := pkgapiexamsessions.NewExamSessionHandler(sm, examServer, repo)
	examTrackingsHandler := pkgapiexamtrackings.NewExamTrackingsHandler(sm, trackingServer)

	mux := http.NewServeMux()
	mux.Handle("/api/examdocs", examHandler)
	mux.Handle("/api/examsessions", examSessionHandler)
	mux.Handle("/api/examsessions/", examSessionHandler)
	mux.Handle("/api/examtrackings", examTrackingsHandler)

	if cli.AssetsDir != "" {
		mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(cli.AssetsDir))))
	}

	mux.Handle("/", dcnaquestions.Handler())

	jwtSec, err := cli.getJWTSecret()
	if err != nil {
		return fmt.Errorf("failed to get JWT secret: %v", err)
	}

	keyProvider := pkgauth.NewStaticSecretProvider(jwtSec)
	var blProvider pkgauth.BlackListProvider
	if blTxtPath := cli.SubjectBlacklistTxtPath; blTxtPath != "" {
		txtblProvider, err := pkgauth.NewTextBasedBlackListProvider(blTxtPath)
		if err != nil {
			return fmt.Errorf("failed to load blacklist file: %v", err)
		}
		blProvider = txtblProvider
	} else {
		blProvider = pkgauth.NewNullBlackListProvider()
	}
	jwtValidator := pkgauth.NewStaticKeyJWTValidator(keyProvider, blProvider, cli.RejectVisitor)

	var h http.Handler = mux


	var authRejectHandler http.Handler = nil

	logger.Info("listening", "addr", cli.Addr)

	whList := []string{
		"/api/login",
		"/api/login/",
		"/api/logout",
	}

	h = pkglog.WithSessionAwaredLog(logger, sm, h)
	h = pkgsession.WithSessionId(h, sm)
	h = pkgauth.WithWhiteListJWTAuth(h, jwtValidator, whList, authRejectHandler)
	h = pkglog.WithHTTPLog(logger, h)
	h = pkglog.WithOverallLog(logger, h)
	h = pkglog.WithLogTraceId(h)

	return http.ListenAndServe(cli.Addr, h)
}

func (hubCmd *CLI) getJWTSecret() ([]byte, error) {
	return getJWTSecFromSomewhere(hubCmd.JWTAuthSecretFromEnv, hubCmd.JWTAuthSecretFromFile)
}

func main() {
	var cli CLI
	ctx := kong.Parse(&cli)
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}

func getJWTSecFromSomewhere(envVar string, filePath string) ([]byte, error) {
	if envVar != "" {
		secret := os.Getenv(envVar)
		if secret == "" {
			return nil, fmt.Errorf("JWT secret is not set in environment variable %s", envVar)
		}
		return []byte(secret), nil
	}

	if filePath != "" {
		secret, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read JWT secret file %s: %v", filePath, err)
		}
		if len(secret) == 0 {
			return nil, fmt.Errorf("JWT secret file %s is empty", filePath)
		}
		return secret, nil
	}

	return nil, fmt.Errorf("no JWT secret is set")
}
