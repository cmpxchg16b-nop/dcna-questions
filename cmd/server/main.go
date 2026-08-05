package main

import (
	dcnaquestions "dcna-questions"
	pkgapiexamdocs "dcna-questions/pkg/api/examdocs"
	pkgapiexamsessions "dcna-questions/pkg/api/examsessions"
	pkgapiexamtrackings "dcna-questions/pkg/api/examtrackings"
	pkgapiloginvisitor "dcna-questions/pkg/api/login/visitor"
	pkgapilogout "dcna-questions/pkg/api/logout"
	pkgapiprofile "dcna-questions/pkg/api/profile"
	pkgauth "dcna-questions/pkg/auth"
	pkgcookie "dcna-questions/pkg/cookie"
	pkgmodelsexamreport "dcna-questions/pkg/models/examreport"
	pkgmodelsexamserver "dcna-questions/pkg/models/examserver"
	pkgmodelsquestion "dcna-questions/pkg/models/question"
	pkgsession "dcna-questions/pkg/session"
	"fmt"
	"os"
	"time"

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
	Addr                        string        `name:"addr" help:"Listening address." env:"ADDR" default:":8080"`
	AssetsDir                   string        `name:"assets-dir" help:"Directory of static assets to serve under /assets/." env:"ASSETS_DIR" type:"existingdir"`
	LoadExam                    []string      `name:"load-exam" help:"Paths to exam documents to load." env:"LOAD_EXAM" type:"existingfile"`
	LoadExamDir                 []string      `name:"load-exam-dir" help:"Directories of exam documents to load." env:"LOAD_EXAM_DIR" type:"existingdir"`
	JWTAuthSecretFromEnv        string        `name:"jwt-auth-secret-from-env" help:"Name of the environment variable that contains the JWT secret"`
	JWTAuthSecretFromFile       string        `name:"jwt-auth-secret-from-file" help:"Path to the file that contains the JWT secret"`
	SubjectBlacklistTxtPath     string        `name:"subj-blacklist-path" help:"Path to the blacklist text file, one subject id per a line"`
	RejectVisitor               bool          `name:"reject-visitor" help:"Reject requests from visitors (subjects with the 'visitor:' prefix)" default:"false"`
	VisitorSessionValidity      time.Duration `name:"validity-of-visitor-session" help:"Validity of visitor session" default:"168h"`
	VisitorSessionTicketGenIntv time.Duration `name:"visitor-jwt-ticket-gen-intv" help:"We issue visitor token based on some ticket generator, this is the interval of how fast it generate tickets" default:"1s"`
	JWTIssuer                   string        `help:"The issuer of the JWT token" default:"exam-server"`
}

func (cli *CLI) Run() error {
	ctx := context.Background()

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

	muxHandlerDyn := http.NewServeMux()
	muxHandlerDyn.Handle("/api/examdocs", examHandler)
	muxHandlerDyn.Handle("/api/profile", pkgapiprofile.NewProfileHandler())
	// /api/logout is on the JWT whitelist below, so the handler also runs for
	// requests whose token is already expired or invalid — clearing cookies
	// must never depend on a still-valid session.
	muxHandlerDyn.Handle("/api/logout", pkgapilogout.NewLogoutHandler(""))
	muxHandlerDyn.Handle("/api/examsessions", examSessionHandler)
	muxHandlerDyn.Handle("/api/examsessions/", examSessionHandler)
	muxHandlerDyn.Handle("/api/examtrackings", examTrackingsHandler)

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
	tokenIssuer := pkgauth.NewStaticKeyJWTIssuer(keyProvider, cli.JWTIssuer)
	tickIssuer := pkgauth.NewSharedTickingTicketGenerator(cli.VisitorSessionTicketGenIntv)
	tickIssuer.Run(ctx)
	cookieBuilder := &pkgcookie.SimpleCookieBuilder{}
	visitorLoginHandler := pkgapiloginvisitor.NewVisitorLoginHandler(
		tokenIssuer,
		cli.VisitorSessionValidity,
		tickIssuer,
		cookieBuilder,
	)

	muxHandlerDyn.Handle("/api/login/visitor", visitorLoginHandler)

	if cli.AssetsDir != "" {
		muxHandlerDyn.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(cli.AssetsDir))))
	}

	muxHandlerStatic := http.NewServeMux()
	muxHandlerStatic.Handle("/", dcnaquestions.Handler())

	jwtValidator := pkgauth.NewStaticKeyJWTValidator(keyProvider, blProvider, cli.RejectVisitor)

	var authRejectHandler http.Handler = nil

	whList := []string{
		"/api/login",
		"/api/login/",
		"/api/logout",
	}

	// Detailed per-request middleware applies only to dynamic (api + assets) endpoints.
	var dynHandler http.Handler = muxHandlerDyn
	dynHandler = pkglog.WithSessionAwaredLog(logger, sm, dynHandler)
	dynHandler = pkgsession.WithSessionId(dynHandler, sm)
	dynHandler = pkgauth.WithWhiteListJWTAuth(dynHandler, jwtValidator, whList, authRejectHandler)
	dynHandler = pkglog.WithHTTPLog(logger, dynHandler)

	muxHandlerGeneral := http.NewServeMux()
	muxHandlerGeneral.Handle("/api/", dynHandler)
	muxHandlerGeneral.Handle("/assets/", dynHandler)
	muxHandlerGeneral.Handle("/", muxHandlerStatic)

	// Trace id and overall log wrap the general mux so both static and dynamic
	// requests get a trace id and an overall log line; dynamic requests
	// additionally get the detailed HTTP log above.
	var h http.Handler = muxHandlerGeneral
	h = pkglog.WithOverallLog(logger, h)
	h = pkglog.WithLogTraceId(h)

	logger.Info("listening", "addr", cli.Addr)

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
