package components

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.ytsaurus.tech/yt/go/yt"

	mock_yt "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/mock"
)

func TestCreateUser_IssuesToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock_yt.NewMockClient(ctrl)
	ctx := context.Background()

	userName := "test_user"
	token := "some_token"
	expectedIssuedToken := "ytct-1234-5678-abcd"

	// Expect CreateObject to be called for user creation
	mockClient.EXPECT().
		CreateObject(ctx, yt.NodeUser, gomock.Any()).
		Return(yt.NodeID{}, nil)

	// Expect IssueToken to be called instead of CreateNode and SetNode for cypress_tokens
	mockClient.EXPECT().
		IssueToken(ctx, userName, "", nil).
		Return(expectedIssuedToken, nil)

	// Call CreateUser with a token
	issuedToken, err := CreateUser(ctx, mockClient, userName, token, false)

	require.NoError(t, err)
	require.Equal(t, expectedIssuedToken, issuedToken)
}

func TestCreateUser_NoToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock_yt.NewMockClient(ctrl)
	ctx := context.Background()

	userName := "test_user"

	// Expect CreateObject to be called for user creation
	mockClient.EXPECT().
		CreateObject(ctx, yt.NodeUser, gomock.Any()).
		Return(yt.NodeID{}, nil)

	// IssueToken should NOT be called when token is empty

	// Call CreateUser without a token
	issuedToken, err := CreateUser(ctx, mockClient, userName, "", false)

	require.NoError(t, err)
	require.Equal(t, "", issuedToken)
}

func TestCreateUser_Superuser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock_yt.NewMockClient(ctrl)
	ctx := context.Background()

	userName := "test_superuser"
	token := "some_token"
	expectedIssuedToken := "ytct-abcd-1234-efgh"

	// Expect CreateObject to be called for user creation
	mockClient.EXPECT().
		CreateObject(ctx, yt.NodeUser, gomock.Any()).
		Return(yt.NodeID{}, nil)

	// Expect IssueToken to be called
	mockClient.EXPECT().
		IssueToken(ctx, userName, "", nil).
		Return(expectedIssuedToken, nil)

	// Expect AddMember to be called for superuser
	mockClient.EXPECT().
		AddMember(ctx, "superusers", userName, nil).
		Return(nil)

	// Call CreateUser as superuser
	issuedToken, err := CreateUser(ctx, mockClient, userName, token, true)

	require.NoError(t, err)
	require.Equal(t, expectedIssuedToken, issuedToken)
}
