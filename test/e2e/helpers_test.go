package controllers_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/utils/ptr"
	ctrlcli "sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yterrors"
)

func getComponentPods(ctx context.Context, namespace string) map[string]corev1.Pod {
	podlist := corev1.PodList{}
	noJobPodsReq, err := labels.NewRequirement("job-name", selection.DoesNotExist, []string{})
	Expect(err).Should(Succeed())
	selector := labels.NewSelector()
	selector = selector.Add(*noJobPodsReq)
	err = k8sClient.List(
		ctx,
		&podlist,
		ctrlcli.InNamespace(namespace),
		ctrlcli.MatchingLabelsSelector{Selector: selector},
	)
	Expect(err).Should(Succeed())

	result := make(map[string]corev1.Pod)
	for _, pod := range podlist.Items {
		result[pod.Name] = pod
	}
	return result
}

func getAllPods(ctx context.Context, namespace string) []corev1.Pod {
	podList := corev1.PodList{}
	err := k8sClient.List(ctx, &podList, ctrlcli.InNamespace(namespace))
	Expect(err).Should(Succeed())
	return podList.Items
}

type changedObjects struct {
	Deleted, Updated, Created []string
}

func getChangedPods(before, after map[string]corev1.Pod) changedObjects {
	var ret changedObjects
	for name := range before {
		if _, found := after[name]; !found {
			ret.Deleted = append(ret.Deleted, name)
		}
	}
	for name, podAfter := range after {
		if !podAfter.DeletionTimestamp.IsZero() {
			ret.Deleted = append(ret.Deleted, name)
		} else if podBefore, found := before[name]; !found {
			ret.Created = append(ret.Created, name)
		} else if !podAfter.CreationTimestamp.Equal(&podBefore.CreationTimestamp) {
			ret.Updated = append(ret.Updated, name)
		}
	}
	return ret
}

// updateSpecToTriggerAllComponentUpdate is a helper
// that introduce spec change which should trigger change in all component static configs
// and thus trigger all components update.
func updateSpecToTriggerAllComponentUpdate(ytsaurus *ytv1.Ytsaurus) {
	// ForceTCP is a field that is used in all components static configs
	// so changing it should trigger all components update.
	// Any value should be ok for test purposes.
	if ytsaurus.Spec.ForceTCP == nil {
		ytsaurus.Spec.ForceTCP = ptr.To(true)
	} else {
		ytsaurus.Spec.ForceTCP = ptr.To(!*ytsaurus.Spec.ForceTCP)
	}
}

type ClusterHealthReport struct {
	Alerts map[ypath.Path][]yterrors.Error
}

func (c ClusterHealthReport) IgnoreAlert(alert yterrors.Error) bool {
	// FIXME(khlebnikov): Fix configs.
	ignoredMessages := []string{
		"Found unrecognized options in dynamic cluster config",
		"Too few matching agents",
		"Conflicting profiling tags",
		"Snapshot loading is disabled; consider enabling it using the controller agent config",
		"Watcher failed to take lock", // scheduler transient alert - read-only cypress
	}
	for _, msg := range ignoredMessages {
		if strings.Contains(alert.Message, msg) {
			return true
		}
	}
	return false
}

func (c ClusterHealthReport) AddAlert(alert yterrors.Error, alertPath ypath.Path) {
	if c.IgnoreAlert(alert) {
		log.Info("Ignoring cluster alert", "alert_path", alertPath, "alert", alert)
	} else {
		log.Error(&alert, "Cluster alert", "alert_path", alertPath)
		c.Alerts[alertPath] = append(c.Alerts[alertPath], alert)
	}
}

func (c ClusterHealthReport) CollectAlerts(ytClient yt.Client, alertPath ypath.Path) {
	var alerts []yterrors.Error
	err := ytClient.GetNode(ctx, alertPath, &alerts, nil)
	if yterrors.ContainsResolveError(err) {
		log.Error(err, "Cannot collect alerts", "alert_path", alertPath)
		return
	}
	Expect(err).Should(Succeed())
	for _, alert := range alerts {
		c.AddAlert(alert, alertPath)
	}
}

func (c ClusterHealthReport) CollectNodes(ytClient yt.Client, basePath ypath.Path) {
	type nodeAlerts struct {
		Name   string           `yson:",value"`
		Alerts []yterrors.Error `yson:"alerts,attr"`
	}
	var nodes []nodeAlerts
	Expect(ytClient.ListNode(ctx, basePath, &nodes, &yt.ListNodeOptions{
		Attributes: []string{"alerts"},
	})).Should(Succeed())
	for _, node := range nodes {
		alertPath := basePath.Child(node.Name).Attr("alerts")
		for _, alert := range node.Alerts {
			c.AddAlert(alert, alertPath)
		}
	}
}

func (c ClusterHealthReport) CollectLostChunks(ytClient yt.Client, countPath ypath.Path) {
	var count int
	Expect(ytClient.GetNode(ctx, countPath, &count, nil)).Should(Succeed())
	if count != 0 {
		alert := yterrors.Error{
			Code:    yterrors.CodeChunkIsLost,
			Message: "Lost chunks",
			Attributes: map[string]any{
				"count_path": countPath,
				"count":      count,
			},
		}
		c.AddAlert(alert, countPath)
	}
}

func CollectClusterHealth(ytClient yt.Client) ClusterHealthReport {
	c := ClusterHealthReport{
		Alerts: map[ypath.Path][]yterrors.Error{},
	}
	c.CollectAlerts(ytClient, "//sys/@master_alerts")
	c.CollectAlerts(ytClient, "//sys/scheduler/@alerts")
	c.CollectLostChunks(ytClient, "//sys/lost_chunks/@count")
	c.CollectLostChunks(ytClient, "//sys/lost_vital_chunks/@count")
	c.CollectNodes(ytClient, "//sys/data_nodes")
	c.CollectNodes(ytClient, "//sys/tablet_nodes")
	c.CollectNodes(ytClient, "//sys/exec_nodes")
	c.CollectNodes(ytClient, "//sys/cluster_nodes")
	c.CollectNodes(ytClient, "//sys/controller_agents/instances")
	return c
}

func queryClickHouse(ytProxyAddress, query string) (string, error) {
	// See https://ytsaurus.tech/docs/en/user-guide/data-processing/chyt/try-chyt
	url := fmt.Sprintf("http://%s/chyt?chyt.clique_alias=ch_public", ytProxyAddress)
	request, err := http.NewRequest(http.MethodPost, url, strings.NewReader(query))
	if err != nil {
		return "", err
	}
	request.Header.Add("Authorization", "OAuth "+consts.DefaultAdminPassword)

	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	content, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	if response.StatusCode != http.StatusOK {
		err := fmt.Errorf("Status: %v Headers: %v Content: %v", response.Status, response.Header, string(content))
		return string(content), err
	}

	return string(content), nil
}
