package components

import (
	"testing"
)

func TestYtsaurusClientFlow(t *testing.T) {
	namespace := "ytclient"
	h, ytsaurus, cfgen := prepareTest(t, namespace)
	defer h.Stop()

	component := NewYtsaurusClient(cfgen, ytsaurus, nil)
	syncUntilReady(t, h, component)

	// TODO: test update flow.
}
