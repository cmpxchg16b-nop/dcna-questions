package utils

type CtxKey string

const (
	// A session id is a globally unique identifier assign to each session
	CtxKeySessionId               = CtxKey("session_id")
	CtxKeySessionTTLSecs = CtxKey("session_ttl_secs")
	CtxKeySessionObject  = CtxKey("session_object")

	// A subject id is a globally unique identifier use for identifying the user
	CtxKeySubjectId               = CtxKey("subject_id")

	// A log trace id is a random string assigned to each http request use for correlate log statements across multiple location
	CtxLogTraceId = CtxKey("log_trace_id")

	// See usage
	CtxKeyJustIssuedJWTToken      = CtxKey("just_issued_jwt_token")

	// See usage
	CtxKeyJWTSecret               = CtxKey("jwt_secret")

	// See usage
	CtxKeyUsername                = CtxKey("username")

	// See usage
	CtxKeyEmail                   = CtxKey("email")
)
