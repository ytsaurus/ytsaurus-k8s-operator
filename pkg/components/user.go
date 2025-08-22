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

func createUserCommand(userName, password, token string, isSuperuser bool) Script {
	var script Script
	script.Append(fmt.Sprintf("/usr/bin/yt create user --attributes '{name=\"%s\"}' --ignore-existing", userName))

	if password != "" {
		passwordHash := sha256String(password)
		script.Append(fmt.Sprintf("/usr/bin/yt execute set_user_password '{user=%s;new_password_sha256=\"%s\"}'", userName, passwordHash))
	}

	if token != "" {
		tokenHash := sha256String(token)
		script.Append(fmt.Sprintf("/usr/bin/yt create map_node '//sys/cypress_tokens/%s' --ignore-existing", tokenHash))
		script.Append(fmt.Sprintf("/usr/bin/yt set '//sys/cypress_tokens/%s/@user' '%s'", tokenHash, userName))
	}

	if isSuperuser {
		script.Append(fmt.Sprintf("/usr/bin/yt add-member %s superusers || true", userName))
	}

	return script
}
