package components

import (
	"crypto/sha256"
	"fmt"
)

func sha256String(value string) string {
	hash := sha256.New()
	// TODO(psushin): handle errors.
	n, err := hash.Write([]byte(value))
	if err != nil || n != len([]byte(value)) {
		panic("cannot hash string")
	}
	bs := hash.Sum(nil)
	return fmt.Sprintf("%x", bs)
}

func createUserCommand(userName, password, token string, isSuperuser bool) []string {
	result := []string{
		fmt.Sprintf("/usr/bin/yt create user --attributes '{name=\"%s\"}' --ignore-existing", userName),
	}

	if password != "" {
		passwordHash := sha256String(password)
		result = append(result, fmt.Sprintf("/usr/bin/yt execute set_user_password '{user=%s;new_password_sha256=\"%s\"}'", userName, passwordHash))
	}

	// Note: Token issuance is now handled via the issue-token API in the Go code,
	// not through shell commands. The cypress_tokens manual registration has been removed
	// in favor of using YTsaurus's standard token issuance mechanism.

	if isSuperuser {
		result = append(result, fmt.Sprintf("/usr/bin/yt add-member %s superusers || true", userName))
	}

	return result
}
