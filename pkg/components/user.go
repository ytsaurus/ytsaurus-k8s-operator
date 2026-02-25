package components

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yterrors"
)

const (
	// YTsaurus tokens follows format "ytct-{4}-{32}".
	tokenPrefixPrefix  = "ytct-"
	tokenPrefixLength  = 10
	tokenMinimalLength = 40

	tokenHashPrefixLength = 8

	// Bootstrap password and token for issuing YTsaurus token via API.
	bootstrapTokenLength    = 30
	bootstrapPasswordPrefix = "yt-bootstrap-password-" //nolint:gosec //not a secret
	bootstrapTokenPrefix    = "yt-bootstrap-token-"    //nolint:gosec //not a secret
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

func hashedTokenPrefix(hash string) string {
	return hash[0:min(len(hash)/2, tokenHashPrefixLength)] + "..."
}

func createUserCommand(userName, password, token string, isSuperuser bool) []string {
	result := []string{
		fmt.Sprintf("/usr/bin/yt create user --attributes '{name=\"%s\"}' --ignore-existing", userName),
	}

	if password != "" {
		passwordHash := sha256String(password)
		result = append(result, fmt.Sprintf("/usr/bin/yt execute set_user_password '{user=%s;new_password_sha256=\"%s\"}'", userName, passwordHash))
	}

	if token != "" {
		tokenHash := sha256String(token)
		result = append(result, fmt.Sprintf("/usr/bin/yt create map_node '//sys/cypress_tokens/%s' --ignore-existing", tokenHash))
		result = append(result, fmt.Sprintf("/usr/bin/yt set '//sys/cypress_tokens/%s/@user' '%s'", tokenHash, userName))
	}

	if isSuperuser {
		result = append(result, fmt.Sprintf("/usr/bin/yt add-member %s superusers || true", userName))
	}

	return result
}

func createUser(ctx context.Context, yc yt.Client, userName, groupName, initToken string) (token string, err error) {
	logger := log.FromContext(ctx)
	logger.Info("Creating user", "userName", userName, "groupName", groupName)

	userID, err := yc.CreateObject(ctx, yt.NodeUser, &yt.CreateObjectOptions{
		IgnoreExisting: true,
		Attributes: map[string]any{
			"name": userName,
		}})
	if err != nil {
		return "", err
	}

	if groupName != "" {
		err = yc.AddMember(ctx, groupName, userName, nil)
		if err != nil && !yterrors.ContainsErrorCode(err, yterrors.CodeAlreadyPresentInGroup) {
			return "", err
		}
	}

	if tokens, err := yc.ListUserTokens(ctx, userName, "", nil); err != nil {
		return "", err
	} else {
		// Revoke all excess tokens.
		for _, hashedToken := range tokens {
			if initToken != "" && sha256String(initToken) == hashedToken {
				continue
			}
			logger.Info("Revoking user token", "userName", userName, "userID", userID, "hashedToken", hashedTokenPrefix(hashedToken))
			// FIXME(khlebnikov): This API is broken - revoke should not hash token again.
			// err := yc.RevokeToken(ctx, userName, "", hashedToken, nil)
			if err = yc.RemoveNode(ctx, ypath.Path("//sys/cypress_tokens").Child(hashedToken), nil); err != nil {
				logger.Error(err, "Cannot revoke user token", "userName", userName, "userID", userID, "hashedToken", hashedTokenPrefix(hashedToken))
				return "", err
			}
		}
	}

	if initToken != "" {
		// TODO(khlebnikov): Remove this and always issues tokens only via API.
		if _, err := yc.CreateNode(
			ctx,
			ypath.Path("//sys/cypress_tokens").Child(sha256String(initToken)),
			yt.NodeMap,
			&yt.CreateNodeOptions{
				IgnoreExisting: true,
				Attributes: map[string]any{
					"user": userName,
				},
			},
		); err != nil {
			return "", err
		}
		token = initToken
	} else {
		token, err = yc.IssueToken(ctx, userName, "", nil)
		if err != nil {
			return "", err
		}
	}

	tokenPrefix := ""
	if len(token) >= tokenMinimalLength && strings.HasPrefix(token, tokenPrefixPrefix) {
		tokenPrefix = token[:tokenPrefixLength]
	}
	logger.Info("User created", "userName", userName, "userID", userID, "groupName", groupName, "tokenPrefix", tokenPrefix)
	return token, nil
}
