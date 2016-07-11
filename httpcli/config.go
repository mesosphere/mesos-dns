package httpcli

// AuthMechanism enumerates the supported authentication strategies
type AuthMechanism string

const (
	// No authentication mechanism
	AuthNone AuthMechanism = "none"

	// Use HTTP basic
	AuthBasic AuthMechanism = "basic"

	// Use IAM / JWT authentication
	AuthIAM AuthMechanism = "iam"
)
