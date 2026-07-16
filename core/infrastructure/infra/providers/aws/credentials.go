//go:build with_aws
// +build with_aws

package aws

import (
	"os"
)

// CredentialResolution describes how AWS credentials are resolved.
type CredentialResolution string

const (
	// CredEnvVars uses AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY env vars.
	CredEnvVars CredentialResolution = "env"
	// CredSharedCredentials uses ~/.aws/credentials file.
	CredSharedCredentials CredentialResolution = "shared"
	// CredIAMRole uses the EC2 instance's IAM role.
	CredIAMRole CredentialResolution = "iam_role"
	// CredSTSSession uses an STS assume-role session.
	CredSTSSession CredentialResolution = "sts"
)

// DetectCredentialResolution checks which AWS credential mechanism is available.
// Priority: env vars > shared credentials > IAM role
func DetectCredentialResolution() CredentialResolution {
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		return CredEnvVars
	}
	if os.Getenv("AWS_PROFILE") != "" {
		return CredSharedCredentials
	}
	// Check if running on EC2 with IAM role
	if os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI") != "" {
		return CredIAMRole
	}
	return CredSharedCredentials // default to shared credentials
}
