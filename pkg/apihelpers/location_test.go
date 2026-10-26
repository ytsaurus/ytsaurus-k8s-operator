package apihelpers

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

func TestMaxLogsTotalSize(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		policy *ytv1.LogRotationPolicy
		want   string
	}{
		{
			name: "no rotation",
		},
		{
			name:   "no size limits",
			policy: &ytv1.LogRotationPolicy{},
		},
		{
			name: "unknown active segment size",
			policy: &ytv1.LogRotationPolicy{
				MaxTotalSizeToKeep: ptr.To(resource.MustParse("100Mi")),
			},
		},
		{
			name: "unbounded retention",
			policy: &ytv1.LogRotationPolicy{
				MaxSegmentSize: ptr.To(resource.MustParse("10Mi")),
			},
		},
		{
			name: "retained size plus active segment",
			policy: &ytv1.LogRotationPolicy{
				MaxTotalSizeToKeep: ptr.To(resource.MustParse("100Mi")),
				MaxSegmentSize:     ptr.To(resource.MustParse("100Mi")),
			},
			want: "200Mi",
		},
		{
			name: "retained count plus active segment",
			policy: &ytv1.LogRotationPolicy{
				MaxSegmentCountToKeep: ptr.To(int64(3)),
				MaxSegmentSize:        ptr.To(resource.MustParse("10Mi")),
			},
			want: "40Mi",
		},
		{
			name: "tighter total size limit",
			policy: &ytv1.LogRotationPolicy{
				MaxTotalSizeToKeep:    ptr.To(resource.MustParse("20Mi")),
				MaxSegmentCountToKeep: ptr.To(int64(3)),
				MaxSegmentSize:        ptr.To(resource.MustParse("10Mi")),
			},
			want: "30Mi",
		},
		{
			name: "tighter segment count limit",
			policy: &ytv1.LogRotationPolicy{
				MaxTotalSizeToKeep:    ptr.To(resource.MustParse("100Mi")),
				MaxSegmentCountToKeep: ptr.To(int64(3)),
				MaxSegmentSize:        ptr.To(resource.MustParse("10Mi")),
			},
			want: "40Mi",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			original := testCase.policy.DeepCopy()
			got := MaxLogsTotalSize(testCase.policy)
			if testCase.want == "" {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				require.Equal(t, 0, got.Cmp(resource.MustParse(testCase.want)))
			}
			require.Equal(t, original, testCase.policy)
		})
	}
}
