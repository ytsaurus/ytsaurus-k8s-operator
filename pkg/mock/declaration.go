package mock_yt

//go:generate go run go.uber.org/mock/mockgen@v0.5.0 -destination=mock_ytsaurus_client.go -package=mock_yt go.ytsaurus.tech/yt/go/yt Client
