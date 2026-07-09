package components

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/log"

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

func GetTokenPrefix(token string) string {
	if len(token) >= consts.TokenMinimalLength && strings.HasPrefix(token, consts.TokenPrefixPrefix) {
		return token[:consts.TokenPrefixLength]
	}
	return "..."
}

func GetHashedTokenPrefix(hash string) string {
	if len(hash) >= consts.TokenMinimalHashLength {
		return hash[:consts.TokenHashPrefixLength] + "..."
	}
	return "..."
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
		result = append(result, fmt.Sprintf(`while [ "$(/usr/bin/yt check-permission --format json %s administer /)" != '{"action":"allow"}' ] ; do sleep 1 ; done`, userName))
	}

	return result
}

func CreateUserToken(ctx context.Context, yc yt.Client, userName, groupName string) (token string, err error) {
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
		// TODO(khlebnikov): Revoke all excess tokens.
		// - Fix broken API: RevokeToken should not hash token again.
		// - Detect and skip temporary tokens.
		// - Add and get token description - API is missing.
		for _, hashedToken := range tokens {
			err := fmt.Errorf("excess token found for user %s", userName)
			logger.Error(err, "Token revocation is not implemented",
				"userName", userName,
				"userID", userID,
				"hashedTokenPrefix", GetHashedTokenPrefix(hashedToken),
			)
		}
	}

	token, err = yc.IssueToken(ctx, userName, "", nil)
	if err != nil {
		return "", err
	}

	logger.Info("User created",
		"userName", userName,
		"userID", userID,
		"groupName", groupName,
		"tokenPrefix", GetTokenPrefix(token),
	)
	return token, nil
}

func SyncUserToken(
	ctx context.Context,
	client internalYtsaurusClient,
	secret *resources.StringSecret,
	userName string,
	groupName string,
	dry bool,
) (ComponentStatus, error) {
	logger := log.FromContext(ctx)

	if status := client.GetStatus(); !status.IsRunning() {
		return status.Blocker(), nil
	}

	if token, ok := secret.GetValue(consts.TokenSecretKey); ok && secret.GetUserName() == userName {
		if client.shouldSkipCypressOperations() {
			logger.Info("Skipping token validation", "userName", userName)
			return ComponentStatusReadyAfter("Skipping token validation"), nil
		}
		hashedTokens, err := client.GetYtClient().ListUserTokens(ctx, userName, "", nil)
		if err != nil && !yterrors.ContainsErrorCode(err, yterrors.CodeResolveError) {
			return ComponentStatusPending("User token validation"), err
		}
		// TODO(khlebnikov): Detect expired tokens.
		if err == nil && slices.Contains(hashedTokens, sha256String(token)) {
			if len(hashedTokens) > 1 {
				// TODO(khlebnikov): Detect temporary tokens.
				logger.Info("User has more than one token", "userName", userName, "tokensCount", len(hashedTokens))
			}
			return ComponentStatusReadyAfter("User token validated"), nil
		}
		logger.Info("User token need sync", "userName", userName, "tokensCount", len(hashedTokens))
	}

	var err error
	if !dry {
		var token string
		token, err = CreateUserToken(ctx, client.GetYtClient(), userName, groupName)
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

	return ComponentStatusPending("Updating user %s token in %s", userName, secret.Name()), err
}
