package main

import (
	dcnaquestions "dcna-questions"
	pkgapiexamassociations "dcna-questions/pkg/api/examassociations"
	pkgapiexamdocs "dcna-questions/pkg/api/examdocs"
	pkgapiexamsessions "dcna-questions/pkg/api/examsessions"
	pkgapiexamtrackings "dcna-questions/pkg/api/examtrackings"
	pkgapiloginoauth2github "dcna-questions/pkg/api/login/oauth2/github"
	pkgapiloginoidcgeneral "dcna-questions/pkg/api/login/oidc/general"
	pkgapiloginvisitor "dcna-questions/pkg/api/login/visitor"
	pkgapiloginoptions "dcna-questions/pkg/api/loginoptions"
	pkgapilogout "dcna-questions/pkg/api/logout"
	pkgapiprofile "dcna-questions/pkg/api/profile"
	pkgapiuseruploads "dcna-questions/pkg/api/useruploads"
	pkgauth "dcna-questions/pkg/auth"
	pkgcookie "dcna-questions/pkg/cookie"
	pkgmodelsexamreport "dcna-questions/pkg/models/examreport"
	pkgmodelsexamserver "dcna-questions/pkg/models/examserver"
	pkgmodelsmsgnotify "dcna-questions/pkg/models/msgnotify"
	pkgmodelsquestion "dcna-questions/pkg/models/question"
	pkgmodelsserverconfig "dcna-questions/pkg/models/serverconfig"
	pkgmodelsuserexamdocsfsbasedassociation "dcna-questions/pkg/models/userexamdocs/fsbasedassociation"
	pkgmodelsuserupload "dcna-questions/pkg/models/userupload"
	pkgsession "dcna-questions/pkg/session"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"context"
	pkglog "dcna-questions/pkg/log"
	"errors"
	"log/slog"
	"net/http"

	"github.com/alecthomas/kong"
	"github.com/joho/godotenv"
)

// logger is the application-wide structured logger used by the HTTP logging
// middleware. It defaults to slog.Default() (text handler on stderr).
var logger = slog.Default()

type CLI struct {
	Addr                          string        `name:"addr" help:"Listening address." env:"ADDR" default:":8080"`
	AssetsDir                     string        `name:"assets-dir" help:"Directory of static assets to serve under /assets/." env:"ASSETS_DIR" type:"existingdir"`
	LoadExam                      []string      `name:"load-exam" help:"Paths to exam documents to load." env:"LOAD_EXAM" type:"existingfile"`
	LoadExamDir                   []string      `name:"load-exam-dir" help:"Directories of exam documents to load." env:"LOAD_EXAM_DIR" type:"existingdir"`
	ConfigXML                     string        `name:"config-xml" help:"Path to the server configuration XML document (see serverConfig.xsd)." env:"CONFIG_XML" type:"existingfile"`
	JWTAuthSecretFromEnv          string        `name:"jwt-auth-secret-from-env" help:"Name of the environment variable that contains the JWT secret" default:"JWT_SECRET"`
	JWTAuthSecretFromFile         string        `name:"jwt-auth-secret-from-file" help:"Path to the file that contains the JWT secret"`
	SubjectBlacklistTxtPath       string        `name:"subj-blacklist-path" help:"Path to the blacklist text file, one subject id per a line"`
	RejectVisitor                 bool          `name:"reject-visitor" help:"Reject requests from visitors (subjects with the 'visitor:' prefix)" default:"false"`
	VisitorSessionValidity        time.Duration `name:"validity-of-visitor-session" help:"Validity of visitor session" default:"168h"`
	VisitorSessionTicketGenIntv   time.Duration `name:"visitor-jwt-ticket-gen-intv" help:"We issue visitor token based on some ticket generator, this is the interval of how fast it generate tickets" default:"1s"`
	JWTIssuer                     string        `help:"The issuer of the JWT token" default:"exam-server"`
	GithubOAuthClientId           string        `name:"github-oauth-client-id" help:"Github OAuth app client id." env:"GITHUB_OAUTH_CLIENT_ID"`
	GithubOAuthAppSecret          string        `name:"github-oauth-app-secret" help:"Github OAuth app client secret." env:"GITHUB_OAUTH_APP_SECRET"`
	GithubOAuthRedirURL           string        `name:"github-oauth-redir-url" help:"Github OAuth redirect URL." env:"GITHUB_OAUTH_REDIR_URL"`
	GithubOAuthLoginPage          string        `name:"github-oauth-login-page" help:"Github OAuth login/authorize page URL (optional, defaults to Github)." env:"GITHUB_OAUTH_LOGIN_PAGE"`
	GithubOAuthScope              string        `name:"github-oauth-scope" help:"Github OAuth scopes (optional, defaults to read:user)." env:"GITHUB_OAUTH_SCOPE"`
	GithubOAuthTokenEndpoint      string        `name:"github-oauth-token-endpoint" help:"Github OAuth token endpoint URL (optional, defaults to Github)." env:"GITHUB_OAUTH_TOKEN_ENDPOINT"`
	GithubLoginSuccessRedirectURL string        `name:"github-login-success-redirect-url" help:"URL to redirect to after a successful Github login." env:"GITHUB_LOGIN_SUCCESS_REDIRECT_URL"`
	GithubSessionLifespan         time.Duration `name:"github-session-lifespan" help:"Lifespan of the session JWT issued after a Github login." default:"168h"`
	NonceLifespan                 time.Duration `name:"nonce-lifespan" help:"Lifespan of the OAuth nonce." default:"10m"`
	SysadminEmail                 string        `name:"sysadmin-email" help:"Email address of the system administrator, used as the fallback notification destination." env:"SYSADMIN_EMAIL"`
}

func (cli *CLI) Run() error {
	ctx := context.Background()

	// The global server configuration document is loaded once here; its
	// sections are consumed by the wiring below (generic OIDC login
	// providers, the outbound SMTP sender).
	var serverCfg *pkgmodelsserverconfig.ServerConfigXML
	if cli.ConfigXML != "" {
		cfg, err := pkgmodelsserverconfig.LoadServerConfig(cli.ConfigXML)
		if err != nil {
			return err
		}
		serverCfg = cfg
	}

	var sources []pkgmodelsquestion.ExamSource

	sm := pkgsession.NewOnMemorySessionManager()
	userUploadManager := pkgmodelsuserupload.NewOnMemoryUserUploadManager()
	associationManager := pkgmodelsuserexamdocsfsbasedassociation.NewFsBasedAssociationManager(userUploadManager, sm)
	go associationManager.Run(ctx)
	defer associationManager.Shutdown()

	sources = append(sources, associationManager)

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
	examHandler := pkgapiexamdocs.NewExamHandler(sm, repo)

	// A single ExamTrackingServer is shared by the exam server (which persists
	// finished-session reports) and the /api/examtrackings handler (which reads
	// them back), so a report written on session end is immediately visible to
	// the caller.
	// Notifications emitted by the tracking server are handed to the service
	// message hub, whose router selects the next hop by destination address
	// family: console destinations are written to the console sink, and email
	// destinations go to the outbound SMTP sender when one is configured.
	// Destinations with no route (e.g. email without an SMTP sender) are
	// dropped by the hub with a log line.
	msgRoutes := []pkgmodelsmsgnotify.MsgRoute{
		{
			DstAddrFamily: pkgmodelsmsgnotify.MsgNotifyAddrFamilyConsole,
			NextHop:       pkgmodelsmsgnotify.SimpleConsoleMessagingService{},
		},
	}

	// The outbound SMTP sender described by the <smtpServer/> section of the
	// server configuration document. Constructing it here also validates the
	// SMTP settings at startup.
	if serverCfg != nil && serverCfg.SMTPServer != nil {
		smtpSender, err := newEmailBasedMsgSvc(serverCfg.SMTPServer)
		if err != nil {
			return err
		}
		msgRoutes = append(msgRoutes, pkgmodelsmsgnotify.MsgRoute{
			DstAddrFamily: pkgmodelsmsgnotify.MsgNotifyAddrFamilyEmail,
			NextHop:       smtpSender,
		})
		logger.Info("registered SMTP message sink for email destinations",
			"server", serverCfg.SMTPServer.Host, "port", serverCfg.SMTPServer.Port)
	}

	msgRouter := pkgmodelsmsgnotify.NewOnMemoryMsgRouter(msgRoutes)
	msgHub := pkgmodelsmsgnotify.NewServiceMessageHub(msgRouter, cli.SysadminEmail)

	trackingServer := pkgmodelsexamreport.NewOnMemoryExamTrackingServer([]pkgmodelsmsgnotify.MsgNotifySvc{msgHub})
	examServer := pkgmodelsexamserver.NewOnMemoryExamServer(trackingServer, []pkgmodelsmsgnotify.MsgNotifySvc{msgHub})
	go examServer.Run(context.Background())
	defer examServer.Shutdown()

	examSessionHandler := pkgapiexamsessions.NewExamSessionHandler(sm, examServer, repo)
	examTrackingsHandler := pkgapiexamtrackings.NewExamTrackingsHandler(sm, trackingServer)

	muxHandlerDyn := http.NewServeMux()
	muxHandlerDyn.Handle("/api/examdocs", examHandler)
	muxHandlerDyn.Handle("/api/profile", pkgapiprofile.NewProfileHandler(sm))
	// /api/logout is on the JWT whitelist below, so the handler also runs for
	// requests whose token is already expired or invalid — clearing cookies
	// must never depend on a still-valid session.
	muxHandlerDyn.Handle("/api/logout", pkgapilogout.NewLogoutHandler(""))
	muxHandlerDyn.Handle("/api/examsessions", examSessionHandler)
	muxHandlerDyn.Handle("/api/examsessions/", examSessionHandler)
	muxHandlerDyn.Handle("/api/examtrackings", examTrackingsHandler)
	// The subtree registration lets DELETE /api/examtrackings/{id} reach the
	// handler, which parses the report id out of the path itself.
	muxHandlerDyn.Handle("/api/examtrackings/", examTrackingsHandler)
	userUploadsHandler := pkgapiuseruploads.NewUserUploadsHandler(sm, userUploadManager)
	muxHandlerDyn.Handle("/api/useruploads", userUploadsHandler)
	muxHandlerDyn.Handle("/api/useruploads/", userUploadsHandler)
	examAssociationsHandler := pkgapiexamassociations.NewExamAssociationsHandler(sm, associationManager)
	muxHandlerDyn.Handle("/api/examassociations", examAssociationsHandler)
	muxHandlerDyn.Handle("/api/examassociations/{association_id}", examAssociationsHandler)
	muxHandlerDyn.Handle("/api/dyn-assets/uploads/{upload_id}/{vfs_path...}", associationManager)

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

	// The login options endpoint serves the <loginOptions/> section of the
	// server configuration document to the login page. It is registered
	// unconditionally (an empty list when unconfigured) so the frontend can
	// always rely on it; /api/login/ is on the JWT whitelist below, so
	// logged-out visitors can reach it.
	var loginOptions []pkgapiloginoptions.LoginOption
	if serverCfg != nil {
		for _, opt := range serverCfg.LoginOptions.Options {
			loginOptions = append(loginOptions, pkgapiloginoptions.LoginOption{
				Kind:        opt.Kind,
				Name:        opt.Name,
				DisplayName: opt.DisplayName,
				Label:       opt.Label,
				LoginURL:    opt.LoginURL,
			})
		}
	}
	muxHandlerDyn.Handle("/api/login/loginoptions", pkgapiloginoptions.NewLoginOptionsHandler(loginOptions))

	// Github OAuth login. The handler is only wired up when a client id is
	// configured, since the OAuth flow requires app credentials and a redirect
	// URL to function. The nonce issuer is signed with the same static key used
	// for JWT auth.
	if cli.GithubOAuthClientId != "" {
		nonceIssuer := &pkgauth.StaticKeyNonceIssuer{
			NonceLifespan:  cli.NonceLifespan,
			SecretProvider: keyProvider,
		}
		githubLoginHandler := pkgapiloginoauth2github.NewGithubOAuthLoginHandler(
			cli.GithubSessionLifespan,
			cli.GithubOAuthClientId,
			cli.GithubOAuthAppSecret,
			cli.GithubOAuthRedirURL,
			cli.GithubOAuthLoginPage,
			cli.GithubOAuthScope,
			cli.GithubOAuthTokenEndpoint,
			cli.GithubLoginSuccessRedirectURL,
			tokenIssuer,
			nonceIssuer,
			cookieBuilder,
		)
		muxHandlerDyn.Handle("/api/login/oauth2/github", githubLoginHandler)
		muxHandlerDyn.Handle("/api/login/oauth2/github/", githubLoginHandler)
		logger.Info("registered Github login handler", "path", "/api/login/oauth2/github/")
	}

	// Generic OIDC providers loaded from the <oidcLoginOptions/> section of
	// the server configuration document (loaded above). Each
	// <oidcLoginOption/> with a non-empty issuerURL registers a
	// GenericOIDCLoginHandler at /api/login/oidc/{providerName}[/...].
	// Entries with an empty issuerURL are skipped so the shipped sample file
	// can be used as-is.
	if serverCfg != nil {
		nonceIssuer := &pkgauth.StaticKeyNonceIssuer{
			NonceLifespan:  cli.NonceLifespan,
			SecretProvider: keyProvider,
		}
		for _, opt := range serverCfg.OIDCLoginOptions.Options {
			if opt.IssuerURL == "" {
				continue
			}
			providerName := opt.ProviderName
			if providerName == "" {
				providerName = "oidc"
			}
			sessionLifespan, err := pkgmodelsserverconfig.ParseSessionLifespan(opt.SessionLifespan, 168*time.Hour)
			if err != nil {
				return fmt.Errorf("OIDC provider %q: %w", providerName, err)
			}
			handler := pkgapiloginoidcgeneral.NewGenericOIDCLoginHandler(
				sessionLifespan,
				opt.ProviderName,
				opt.IssuerURL,
				opt.ClientId,
				opt.ClientSecret,
				opt.RedirectURL,
				opt.Scope,
				opt.LoginSuccessRedirectURL,
				tokenIssuer,
				nonceIssuer,
				cookieBuilder,
			)
			base := "/api/login/oidc/" + providerName
			muxHandlerDyn.Handle(base, handler)
			muxHandlerDyn.Handle(base+"/", handler)
			logger.Info("registered OIDC login handler", "provider", providerName, "path", base+"/", "issuer", opt.IssuerURL)
		}
	}

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

// dotEnvFiles are the dotenv files loaded at startup, in decreasing order
// of precedence: godotenv.Load never overrides a variable that is already
// set, so .env.local wins over .env, and both lose to the real environment.
var dotEnvFiles = []string{".env.local", ".env"}

// loadDotEnvFiles loads the conventional dotenv files (.env.local, .env)
// into the process environment. It runs before kong.Parse so that kong's
// env-tagged CLI fields observe the variables defined there. Missing files
// are skipped; a failure to load an existing file is fatal.
func loadDotEnvFiles() {
	var existing []string
	for _, f := range dotEnvFiles {
		if _, err := os.Stat(f); err == nil {
			existing = append(existing, f)
		}
	}
	if len(existing) == 0 {
		return
	}
	if err := godotenv.Load(existing...); err != nil {
		logger.Error("failed to load dot env files", "files", existing, "err", err)
		os.Exit(1)
	}
	logger.Info("loaded dot env files", "files", existing)
}

func main() {
	loadDotEnvFiles()

	var cli CLI
	ctx := kong.Parse(&cli)
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}

// newEmailBasedMsgSvc builds the outbound SMTP message sender described by
// the <smtpServer/> section of the server configuration document.
func newEmailBasedMsgSvc(cfg *pkgmodelsserverconfig.SMTPServerXML) (*pkgmodelsmsgnotify.EmailBasedMsgSvc, error) {
	encryption := pkgmodelsmsgnotify.SMTPEncryptionNone
	switch {
	case cfg.StartTLS && cfg.TLS:
		return nil, errors.New("smtpServer: startTLS and tls are mutually exclusive")
	case cfg.StartTLS:
		encryption = pkgmodelsmsgnotify.SMTPEncryptionStartTLS
	case cfg.TLS:
		encryption = pkgmodelsmsgnotify.SMTPEncryptionTLS
	}
	return pkgmodelsmsgnotify.NewEmailBasedMsgSvc(pkgmodelsmsgnotify.EmailBasedMsgSvcInitOption{
		ServerAddr: net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Encryption: encryption,
		Username:   cfg.Username,
		Password:   cfg.Password,
	})
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
