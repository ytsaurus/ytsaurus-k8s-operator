package controllers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/component-helpers/resource"
	"k8s.io/utils/ptr"
	ctrlcli "sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"

	"go.ytsaurus.tech/yt/go/guid"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yson"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yterrors"
)

func YsonPretty(value any) string {
	data, err := yson.MarshalFormat(value, yson.FormatPretty)
	Expect(err).To(Succeed())
	return string(data)
}

func getKindControlPlaneNode(ctx context.Context) corev1.Node {
	nodeList := corev1.NodeList{}
	err := k8sClient.List(ctx, &nodeList)
	Expect(err).Should(Succeed())
	Expect(nodeList.Items).To(HaveLen(1))
	Expect(nodeList.Items[0].Name).To(HaveSuffix("kind-control-plane"))
	return nodeList.Items[0]
}

func logNodesState(ctx context.Context) {
	var nodes corev1.NodeList
	var pods corev1.PodList
	Expect(k8sClient.List(ctx, &nodes)).Should(Succeed())
	Expect(k8sClient.List(ctx, &pods)).Should(Succeed())
	for _, node := range nodes.Items {
		available := node.Status.Allocatable.DeepCopy()
		for _, pod := range pods.Items {
			if pod.Spec.NodeName != node.Name {
				continue
			}
			requests := resource.PodRequests(&pod, resource.PodResourcesOptions{})
			log.Info("Pod",
				"namespace", pod.Namespace,
				"name", pod.Name,
				"qosClass", pod.Status.QOSClass,
				"phase", pod.Status.Phase,
				"reason", pod.Status.Reason,
				"message", pod.Status.Message,
				"requests", requests,
			)
			if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
				continue
			}
			for k, v := range requests {
				res := available[k]
				res.Sub(v)
				available[k] = res
			}
		}
		log.Info("Node",
			"name", node.Name,
			"allocatable", node.Status.Allocatable,
			"available", available,
		)
	}
}

func getNodesAddresses(ctx context.Context) []string {
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

func getServiceAddress(ctx context.Context, namespace, serviceName, portName string) (string, error) {
	svc := corev1.Service{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: namespace}, &svc)
	if err != nil {
		return "", err
	}

	k8sNode := getKindControlPlaneNode(ctx)

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

func getOperatorMetricsURL(ctx context.Context) (string, error) {
	if url := os.Getenv("E2E_YT_OPERATOR_METRICS_URL"); url != "" {
		return url, nil
	}
	namespace := os.Getenv("OPERATOR_NAMESPACE")
	instance := os.Getenv("OPERATOR_INSTANCE")
	chart := os.Getenv("OPERATOR_CHART")
	if namespace == "" || instance == "" || chart == "" {
		return "", fmt.Errorf("operator namespace, instance, or chart is not defined")
	}
	address, err := getServiceAddress(ctx, namespace, instance+"-"+chart+"-metrics", "metrics")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("http://%s/metrics", address), nil
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

func getExpectedUpdatedPods(podsBeforeUpdate map[string]corev1.Pod, stsName string) []string {
	expectedUpdated := make([]string, 0)
	for name := range podsBeforeUpdate {
		if strings.HasPrefix(name, stsName+"-") {
			expectedUpdated = append(expectedUpdated, name)
		}
	}
	return expectedUpdated
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

func (c *ClusterHealthReport) CollectAlerts(ctx context.Context, ytClient yt.Client, alertPath ypath.Path) {
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

func (c *ClusterHealthReport) CollectNodes(ctx context.Context, ytClient yt.Client, basePath ypath.Path) {
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

func (c *ClusterHealthReport) CollectLostChunks(ctx context.Context, ytClient yt.Client, countPath ypath.Path) {
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

func (c *ClusterHealthReport) CollectTablets(ctx context.Context, ytClient yt.Client, basePath ypath.Path, attrs []string) {
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

func (c *ClusterHealthReport) Collect(ctx context.Context, ytClient yt.Client) {
	c.Clear()
	c.CollectAlerts(ctx, ytClient, "//sys/@master_alerts")
	c.CollectAlerts(ctx, ytClient, "//sys/scheduler/@alerts")
	c.CollectLostChunks(ctx, ytClient, "//sys/lost_chunks/@count")
	c.CollectLostChunks(ctx, ytClient, "//sys/lost_vital_chunks/@count")
	c.CollectNodes(ctx, ytClient, "//sys/data_nodes")
	c.CollectNodes(ctx, ytClient, "//sys/tablet_nodes")
	c.CollectNodes(ctx, ytClient, "//sys/exec_nodes")
	c.CollectNodes(ctx, ytClient, "//sys/cluster_nodes")
	c.CollectNodes(ctx, ytClient, "//sys/controller_agents/instances")
	c.CollectTablets(ctx, ytClient, "//sys/tablet_cell_bundles", []string{"id", "health", "tablet_cell_ids", "tablet_actions", "tablet_cell_life_stage"})
	c.CollectTablets(ctx, ytClient, "//sys/tablet_cells", []string{"id", "health", "tablet_cell_bundle", "tablet_cell_life_stage", "status"})
}

type ClusterComponent struct {
	Client  yt.Client
	Type    consts.ComponentType
	Address string
	Orchid  ypath.Path
}

type BusService struct {
	Name      string `yson:"name"`
	Version   string `yson:"version"`
	StartTime string `yson:"start_time"`
	BuildTime string `yson:"build_time"`
	BuildHost string `yson:"build_host"`
}

func (c *ClusterComponent) GetBusService(ctx context.Context) (BusService, error) {
	var result BusService
	err := c.Client.GetNode(ctx, c.Orchid.JoinChild("service"), &result, nil)
	return result, err
}

type BusConnection struct {
	Address          string           `yson:"address"`
	Encrypted        bool             `yson:"encrypted"`
	MultiplexingBand string           `yson:"multiplexing_band"`
	Statistics       map[string]int64 `yson:"statistics"`
}

func (c *ClusterComponent) GetBusConnections(ctx context.Context) (map[string]BusConnection, error) {
	var result map[string]BusConnection
	err := c.Client.GetNode(ctx, c.Orchid.JoinChild("tcp_dispatcher", "connections"), &result, nil)
	return result, err
}

func ListClusterComponents(ctx context.Context, ytClient yt.Client) ([]ClusterComponent, error) {
	var nodes []ClusterComponent
	for _, componentType := range consts.LocalComponentTypes {
		compomentCypressPath := ypath.Path(consts.ComponentCypressPath(componentType))
		if compomentCypressPath == "" {
			continue
		}
		var addresses []string
		if err := ytClient.ListNode(ctx, compomentCypressPath, &addresses, nil); err != nil {
			if yterrors.ContainsResolveError(err) {
				continue
			}
			return nil, err
		}
		for _, address := range addresses {
			nodes = append(nodes, ClusterComponent{
				Client:  ytClient,
				Type:    componentType,
				Address: address,
				Orchid:  compomentCypressPath.JoinChild(address, "orchid"),
			})
		}
	}
	return nodes, nil
}

func queryClickHouse(httpClient *http.Client, ytProxyAddress, query string) (string, error) {
	// See https://ytsaurus.tech/docs/en/user-guide/data-processing/chyt/try-chyt
	url := ytProxyAddress + "/chyt?chyt.clique_alias=ch_public"
	request, err := http.NewRequest(http.MethodPost, url, strings.NewReader(query))
	if err != nil {
		return "", err
	}
	request.Header.Add("Authorization", "OAuth "+consts.DefaultAdminPassword)

	response, err := httpClient.Do(request)
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

func queryClickHouseID(httpClient *http.Client, ytProxyAddress string) (guid.GUID, error) {
	// Actually this is operation id. This is not properly documented feature.
	result, err := queryClickHouse(httpClient, ytProxyAddress, "SELECT clique_id FROM system.clique WHERE self;")
	if err != nil {
		return guid.GUID{}, err
	}
	return guid.ParseString(strings.TrimSpace(result))
}

func makeQuery(ctx context.Context, ytClient yt.Client, engine yt.QueryEngine, query string) []map[string]any {
	id, err := ytClient.StartQuery(ctx, engine, query, nil)
	Expect(err).To(Succeed())
	log.Info("Query started", "id", id)

	var previousState yt.QueryState
	Eventually(func() (yt.QueryState, error) {
		result, err := ytClient.GetQuery(ctx, id, &yt.GetQueryOptions{
			Attributes: []string{"state", "error"},
		})
		if err != nil || result.State == nil {
			return yt.QueryStateFailed, err
		}
		if previousState != *result.State {
			log.Info("Query state", "id", id, "state", *result.State, "error", result.Err)
			previousState = *result.State
		}
		return *result.State, nil
	}, "5m").To(BeElementOf(yt.QueryStateAborted, yt.QueryStateCompleted, yt.QueryStateFailed))

	result, err := ytClient.GetQuery(ctx, id, nil)
	Expect(err).To(Succeed())

	if ptr.Deref(result.ResultCount, 0) == 0 {
		log.Info("Query result", "id", id, "result_count", 0)
		return nil
	}

	Expect(result.State).To(HaveValue(Equal(yt.QueryStateCompleted)))
	Expect(result.ResultCount).To(HaveValue(Equal(int64(1))))

	_, err = ytClient.GetQueryResult(ctx, id, 0, nil)
	Expect(err).To(Succeed())

	reader, err := ytClient.ReadQueryResult(ctx, id, 0, nil)
	Expect(err).To(Succeed())

	var rows []map[string]any
	for reader.Next() {
		var row map[string]any
		Expect(reader.Scan(&row)).To(Succeed())
		rows = append(rows, row)
	}
	Expect(reader.Err()).To(Succeed())
	Expect(reader.Close()).To(Succeed())

	log.Info("Query result", "id", id, "rows", rows)

	return rows
}

func readFileObject(ctx context.Context, namespace string, source ytv1.FileObjectReference) ([]byte, error) {
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

func discoverProxies(httpClient *http.Client, proxyAddress string, params url.Values) []string {
	resp, err := httpClient.Get(proxyAddress + "/api/v4/discover_proxies?" + params.Encode())
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

func fetchStuckPods(ctx context.Context, namespace string) (pending, failed []string) {
	podList := corev1.PodList{}
	err := k8sClient.List(ctx, &podList, ctrlcli.InNamespace(namespace))
	Expect(err).Should(Succeed())

	for _, pod := range podList.Items {
		switch pod.Status.Phase {
		case corev1.PodPending:
			pending = append(pending, pod.Name)
		case corev1.PodFailed:
			failed = append(failed, pod.Name)
		default:
			continue
		}

		requests := resource.PodRequests(&pod, resource.PodResourcesOptions{})
		log.Info("Pod",
			"name", pod.Name,
			"phase", pod.Status.Phase,
			"message", pod.Status.Message,
			"reason", pod.Status.Reason,
			"requests", requests,
			"nodeName", pod.Spec.NodeName,
			"conditions", pod.Status.Conditions,
		)

		if pod.Status.Phase != corev1.PodFailed {
			continue
		}

		logRequest := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
			Timestamps: true,
			TailLines:  ptr.To[int64](20),
		})
		logStream, err := logRequest.Stream(ctx)
		if err != nil {
			log.Error(err, "Cannot get logs")
			continue
		}

		_, err = io.Copy(GinkgoWriter, logStream)
		Expect(logStream.Close()).To(Succeed())
		Expect(err).To(Succeed())
	}

	return pending, failed
}

func pullImages(ctx context.Context, namaspace string, images []string, timeout time.Duration) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namaspace,
			GenerateName: "pull-images-",
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
	for i, image := range images {
		pod.Spec.Containers = append(pod.Spec.Containers,
			corev1.Container{
				Name:            fmt.Sprintf("image%d", i),
				Image:           image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"true"},
			},
		)
	}
	Expect(k8sClient.Create(ctx, &pod)).Should(Succeed())
	EventuallyObject(ctx, &pod, timeout).Should(
		HaveField("Status.Phase", Not(Or(Equal(corev1.PodPending), Equal(corev1.PodRunning)))),
	)
	Expect(pod).Should(HaveField("Status.Phase", Equal(corev1.PodSucceeded)))
	Expect(k8sClient.Delete(ctx, &pod)).Should(Succeed())
}

func reissueCertificate(ctx context.Context, cert *certv1.Certificate) {
	patch := ctrlcli.MergeFrom(cert.DeepCopy())
	cert.Status.Conditions = append(cert.Status.Conditions, certv1.CertificateCondition{
		Type:   certv1.CertificateConditionIssuing,
		Status: certmetav1.ConditionTrue,
	})
	Expect(k8sClient.Status().Patch(ctx, cert, patch)).To(Succeed())

	EventuallyObject(ctx, cert, 10*time.Second).To(Satisfy(func(cert *certv1.Certificate) bool {
		for _, c := range cert.Status.Conditions {
			if c.Type == certv1.CertificateConditionIssuing && c.Status == certmetav1.ConditionTrue {
				return false
			}
		}
		return true
	}))
}

func restartPod(ctx context.Context, namespace, name string) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	Expect(k8sClient.Delete(ctx, pod)).To(Succeed())
	oldUID := pod.UID
	EventuallyObject(ctx, pod, 10*time.Second).To(SatisfyAll(
		HaveField("ObjectMeta.UID", Not(Equal(oldUID))),
		HaveField("Status.Phase", Equal(corev1.PodRunning)),
	))
}
