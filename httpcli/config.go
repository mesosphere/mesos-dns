package httpcli

// AuthMechanism enumerates the supported authentication strategies
type AuthMechanism string

const (
	// AuthNone specifies no authentication mechanism
	AuthNone AuthMechanism = "none"

	// AuthBasic specifies to use HTTP Basic
	AuthBasic AuthMechanism = "basic"

	// AuthIAM specifies to use IAM / JDK authentication
	AuthIAM AuthMechanism = "iam"
)
