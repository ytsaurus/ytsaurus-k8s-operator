package components

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yterrors"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
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

func CreateUser(ctx context.Context, yc yt.Client, userName, groupName, initToken string) (token string, err error) {
	logger := log.FromContext(ctx)
	logger.Info("Creating user", "userName", userName)
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
	} else if len(tokens) != 0 {
		logger.Info("Revoking user tokens", "userName", userName, "userID", userID)
		for _, hashedToken := range tokens {
			if initToken != "" && sha256String(initToken) == hashedToken {
				continue
			}
			// FIXME(khlebnikov): This API is broken - revoke should not hash token.
			err := yc.RevokeToken(ctx, userName, "", hashedToken, nil)
			if err != nil {
				logger.Error(err, "Cannot revoke user token", "userName", userName, "userID", userID, "hashedToken", hashedToken)
			}
		}
	}

	if initToken != "" {
		// TODO(khlebnikov): Remove this and issues via API.
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
	if len(token) >= consts.YtsaurusTokenMinimalLength && strings.HasPrefix(token, consts.YtsaurusTokenPrefix) {
		tokenPrefix = token[:consts.YtsaurusTokenPrefixLength]
	}
	logger.Info("User created", "userName", userName, "userID", userID, "groupName", groupName, "tokenPrefix", tokenPrefix)
	return token, nil
}

func buildUserCredentialsSecretname(username string) string {
	return fmt.Sprintf("%s-secret", username)
}

func syncUserToken(
	ctx context.Context,
	client internalYtsaurusClient,
	secret *resources.StringSecret,
	userName string,
	groupName string,
	dry bool,
) (ComponentStatus, error) {
	logger := log.FromContext(ctx)
	if status, err := client.Status(ctx); err != nil || !status.IsRunning() {
		return ComponentStatusBlockedBy(client.GetFullName()), err
	}
	if token, ok := secret.GetValue(consts.TokenSecretKey); ok && secret.GetUserName() == userName {
		if client.shouldSkipCypressOperations() {
			logger.Info("Skipping token validation", "userName", userName)
			return ComponentStatusReady(), nil
		}
		hashedTokens, err := client.GetYtClient().ListUserTokens(ctx, userName, "", nil)
		if err == nil && slices.Contains(hashedTokens, sha256String(token)) {
			return ComponentStatusReady(), err
		}
		logger.Info("User token need sync", "userName", userName, "tokensCount", len(hashedTokens))
	}
	var err error
	if !dry {
		var token string
		token, err = CreateUser(ctx, client.GetYtClient(), userName, groupName, "")
		if err == nil {
			secret.Build()
			secret.SetUserName(userName)
			secret.SetValue(consts.TokenSecretKey, token)
			if userName == consts.UIUserName {
				secret.SetValue(consts.UISecretFileName, fmt.Sprintf("{\"oauthToken\" : \"%s\"}", token))
			}
			err = secret.Sync(ctx)
		}
	}
	return ComponentStatusPending(fmt.Sprintf("Updating user %s token in %s", userName, secret.Name())), err
}
