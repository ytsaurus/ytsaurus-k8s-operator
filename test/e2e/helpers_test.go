package controllers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrlcli "sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yterrors"
)

func getKindControlPlaneNode() corev1.Node {
	nodeList := corev1.NodeList{}
	err := k8sClient.List(ctx, &nodeList)
	Expect(err).Should(Succeed())
	Expect(nodeList.Items).To(HaveLen(1))
	Expect(nodeList.Items[0].Name).To(HaveSuffix("kind-control-plane"))
	return nodeList.Items[0]
}

func getNodesAddresses() []string {
	var nodes corev1.NodeList
	Expect(k8sClient.List(ctx, &nodes)).Should(Succeed())
	var addrs []string
	for _, node := range nodes.Items {
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeInternalIP && net.ParseIP(address.Address).To4() != nil {
				addrs = append(addrs, address.Address)
				break
			}
		}
	}
	return addrs
}

func getServiceAddress(namespace, serviceName, portName string) (string, error) {
	svc := corev1.Service{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: namespace}, &svc)
	if err != nil {
		return "", err
	}

	k8sNode := getKindControlPlaneNode()

	Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeNodePort))
	Expect(svc.Spec.IPFamilies[0]).To(Equal(corev1.IPv4Protocol))

	Expect(svc.Spec.Ports).To(ContainElement(HaveField("Name", portName)))
	var nodePort int32
	for _, port := range svc.Spec.Ports {
		if port.Name == portName {
			nodePort = port.NodePort
			break
		}
	}
	Expect(nodePort).ToNot(BeZero())

	var nodeAddress string
	for _, address := range k8sNode.Status.Addresses {
		if address.Type == corev1.NodeInternalIP && net.ParseIP(address.Address).To4() != nil {
			nodeAddress = address.Address
			break
		}
	}
	Expect(nodeAddress).ToNot(BeEmpty())

	return fmt.Sprintf("%s:%v", nodeAddress, nodePort), nil
}

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
	Errors   map[ypath.Path][]error
	Alerts   map[ypath.Path][]yterrors.Error
	Warnings map[ypath.Path][]yterrors.Error
}

func (c *ClusterHealthReport) String() string {
	out := new(strings.Builder)
	for k, v := range c.Errors {
		fmt.Fprintf(out, "E %v %v\n", k, v)
	}
	for k, v := range c.Alerts {
		fmt.Fprintf(out, "A %v %v\n", k, v)
	}
	for k, v := range c.Warnings {
		fmt.Fprintf(out, "W %v %v\n", k, v)
	}
	return out.String()
}

func (c *ClusterHealthReport) AddError(err error, errorPath ypath.Path) {
	log.Info("Cluster error", "error", err, "error_path", errorPath)
	if c.Errors == nil {
		c.Errors = make(map[ypath.Path][]error)
	}
	c.Errors[errorPath] = append(c.Errors[errorPath], err)
}

func (c *ClusterHealthReport) IgnoreAlert(alert yterrors.Error) bool {
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

func (c *ClusterHealthReport) AddAlert(alert yterrors.Error, alertPath ypath.Path) {
	if c.IgnoreAlert(alert) {
		log.Info("Ignoring cluster alert", "alert_path", alertPath, "alert", alert)
		return
	}
	log.Info("Cluster alert", "alert", alert, "alert_path", alertPath)
	if c.Alerts == nil {
		c.Alerts = make(map[ypath.Path][]yterrors.Error)
	}
	c.Alerts[alertPath] = append(c.Alerts[alertPath], alert)
}

func (c *ClusterHealthReport) AddWarning(warning yterrors.Error, warningPath ypath.Path) {
	log.Info("Cluster warning", "warning", warning, "warning_path", warningPath)
	if c.Warnings == nil {
		c.Warnings = make(map[ypath.Path][]yterrors.Error)
	}
	c.Warnings[warningPath] = append(c.Warnings[warningPath], warning)
}

func (c *ClusterHealthReport) CollectAlerts(ytClient yt.Client, alertPath ypath.Path) {
	var alerts []yterrors.Error
	if err := ytClient.GetNode(ctx, alertPath, &alerts, nil); err != nil {
		if yterrors.ContainsResolveError(err) {
			log.Info("Cannot collect alerts", "error", err, "alert_path", alertPath)
		} else {
			c.AddError(err, alertPath)
		}
		return
	}
	for _, alert := range alerts {
		c.AddAlert(alert, alertPath)
	}
}

func (c *ClusterHealthReport) CollectNodes(ytClient yt.Client, basePath ypath.Path) {
	type nodeAlerts struct {
		Name   string           `yson:",value"`
		Alerts []yterrors.Error `yson:"alerts,attr"`
	}
	var nodes []nodeAlerts
	err := ytClient.ListNode(ctx, basePath, &nodes, &yt.ListNodeOptions{
		Attributes: []string{"alerts"},
	})
	if err != nil {
		c.AddError(err, basePath)
		return
	}
	for _, node := range nodes {
		alertPath := basePath.Child(node.Name).Attr("alerts")
		for _, alert := range node.Alerts {
			c.AddAlert(alert, alertPath)
		}
	}
}

func (c *ClusterHealthReport) CollectLostChunks(ytClient yt.Client, countPath ypath.Path) {
	var count int
	err := ytClient.GetNode(ctx, countPath, &count, nil)
	if err != nil {
		c.AddError(err, countPath)
		return
	}
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

func (c *ClusterHealthReport) CollectTablets(ytClient yt.Client, basePath ypath.Path, attrs []string) {
	type node struct {
		Name  string         `yson:",value"`
		Attrs map[string]any `yson:",attrs"`
	}
	var nodes []node

	err := ytClient.ListNode(
		ctx,
		basePath,
		&nodes,
		&yt.ListNodeOptions{
			Attributes: attrs,
		},
	)
	if err != nil {
		c.AddError(err, basePath)
		return
	}

	for _, node := range nodes {
		if health, ok := node.Attrs["health"].(string); ok && health != "good" {
			c.AddWarning(
				yterrors.Error{
					Code:       yterrors.CodeGeneric,
					Message:    "Tablet health",
					Attributes: node.Attrs,
				},
				basePath.Child(node.Name),
			)
		}
	}
}

func (c *ClusterHealthReport) Clear() {
	c.Errors = nil
	c.Alerts = nil
	c.Warnings = nil
}

func (c *ClusterHealthReport) Collect(ytClient yt.Client) {
	c.Clear()
	c.CollectAlerts(ytClient, "//sys/@master_alerts")
	c.CollectAlerts(ytClient, "//sys/scheduler/@alerts")
	c.CollectLostChunks(ytClient, "//sys/lost_chunks/@count")
	c.CollectLostChunks(ytClient, "//sys/lost_vital_chunks/@count")
	c.CollectNodes(ytClient, "//sys/data_nodes")
	c.CollectNodes(ytClient, "//sys/tablet_nodes")
	c.CollectNodes(ytClient, "//sys/exec_nodes")
	c.CollectNodes(ytClient, "//sys/cluster_nodes")
	c.CollectNodes(ytClient, "//sys/controller_agents/instances")
	c.CollectTablets(ytClient, "//sys/tablet_cell_bundles", []string{"id", "health", "tablet_cell_ids", "tablet_actions", "tablet_cell_life_stage"})
	c.CollectTablets(ytClient, "//sys/tablet_cells", []string{"id", "health", "tablet_cell_bundle", "tablet_cell_life_stage", "status"})
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
	defer func() {
		Expect(response.Body.Close()).To(Succeed())
	}()

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

func readFileObject(namespace string, source ytv1.FileObjectReference) ([]byte, error) {
	objectName := types.NamespacedName{
		Namespace: namespace,
		Name:      source.Name,
	}
	switch source.Kind {
	case "", "ConfigMap":
		var object corev1.ConfigMap
		if err := k8sClient.Get(ctx, objectName, &object); err != nil {
			return nil, err
		}
		if data, ok := object.Data[source.Key]; ok {
			return []byte(data), nil
		}
		if data, ok := object.BinaryData[source.Key]; ok {
			return data, nil
		}
	case "Secret":
		var object corev1.Secret
		if err := k8sClient.Get(ctx, objectName, &object); err != nil {
			return nil, err
		}
		if data, ok := object.Data[source.Key]; ok {
			return data, nil
		}
		if data, ok := object.StringData[source.Key]; ok {
			return []byte(data), nil
		}
	}
	return nil, fmt.Errorf("Key %v not found in %v/%v", source.Key, source.Kind, source.Name)
}

func discoverProxies(proxyAddress string, params url.Values) []string {
	resp, err := http.Get(proxyAddress + "/api/v4/discover_proxies?" + params.Encode())
	Expect(err).NotTo(HaveOccurred())
	defer func() {
		Expect(resp.Body.Close()).To(Succeed())
	}()
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	var proxies struct {
		Proxies []string `json:"proxies"`
	}
	Expect(json.NewDecoder(resp.Body).Decode(&proxies)).To(Succeed())
	return proxies.Proxies
}
