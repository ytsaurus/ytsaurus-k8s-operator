package ytflow

import (
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

var _ ytsaurusClient = (*components.YtsaurusClient)(nil)
