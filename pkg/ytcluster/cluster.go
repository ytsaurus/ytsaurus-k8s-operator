package ytcluster

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

// Cluster extends Ytsaurus with components defined in separate objects.
type Cluster struct {
	*ytv1.Ytsaurus

	UpdatePlanObject *ytv1.YtsaurusUpdatePlan

	Objects map[ytv1.Component]ComponentObject

	Components map[ytv1.Component]ytv1.ComponentSpec

	ExecNodes   []*ytv1.ExecNodesSpec
	DataNodes   []*ytv1.DataNodesSpec
	TabletNodes []*ytv1.TabletNodesSpec
	HTTPProxies []*ytv1.HTTPProxiesSpec
	RPCProxies  []*ytv1.RPCProxiesSpec
}

type ComponentObject interface {
	client.Object

	GetYtsaurusName() string
	GetStatusObservedGeneration() int64
	SetStatusObservedGeneration(generation int64)
	GetComponentSpec() ytv1.ComponentSpec
	GetComponentStatus() ytv1.ComponentStatus
	SetComponentStatus(status ytv1.ComponentStatus)
}

func KnownYtsaurusComponentObjects() []ComponentObject {
	return []ComponentObject{
		&ytv1.YtsaurusUpdatePlan{},
		&ytv1.YtsaurusExecNodes{},
		&ytv1.YtsaurusDataNodes{},
		&ytv1.YtsaurusTabletNodes{},
		&ytv1.YtsaurusHTTPProxies{},
		&ytv1.YtsaurusRPCProxies{},
	}
}

func ptrs[T any](in []T) []*T {
	out := make([]*T, len(in))
	for i := range in {
		out[i] = &in[i]
	}
	return out
}

func NewCluster(ytsaurus *ytv1.Ytsaurus) *Cluster {
	cluster := &Cluster{
		Ytsaurus:    ytsaurus,
		Objects:     map[ytv1.Component]ComponentObject{},
		Components:  map[ytv1.Component]ytv1.ComponentSpec{},
		ExecNodes:   ptrs(ytsaurus.Spec.ExecNodes),
		DataNodes:   ptrs(ytsaurus.Spec.DataNodes),
		TabletNodes: ptrs(ytsaurus.Spec.TabletNodes),
		HTTPProxies: ptrs(ytsaurus.Spec.HTTPProxies),
		RPCProxies:  ptrs(ytsaurus.Spec.RPCProxies),
	}
	for _, spec := range ytsaurus.GetComponentSpecs() {
		cluster.Components[spec.GetComponent()] = spec
	}
	return cluster
}

func componentGenerationKey(component ytv1.Component, object ComponentObject) string {
	return component.String() + "@" + string(object.GetUID())
}

func (c *Cluster) AreComponentGenerationsObserved() bool {
	observed := c.Status.ObservedComponentGenerations
	if len(observed) != len(c.Objects) {
		return false
	}
	for component, object := range c.Objects {
		if generation, found := observed[componentGenerationKey(component, object)]; !found || generation != object.GetGeneration() {
			return false
		}
	}
	return true
}

func (c *Cluster) UpdateObservedComponentGenerations() {
	generations := make(map[string]int64, len(c.Objects))
	for component, object := range c.Objects {
		generations[componentGenerationKey(component, object)] = object.GetGeneration()
	}
	c.Status.ObservedComponentGenerations = generations
}

func (c *Cluster) AddComponentObject(object ComponentObject) error {
	component := object.GetComponentSpec().GetComponent()
	if _, found := c.Objects[component]; found {
		return fmt.Errorf("duplicate component: %v", component)
	}
	if _, found := c.Components[component]; found {
		return fmt.Errorf("duplicate component: %v", component)
	}
	c.Objects[component] = object
	return nil
}

func (c *Cluster) DeleteComponentObject(object ComponentObject) error {
	component := object.GetComponentSpec().GetComponent()
	if object.GetNamespace() != c.GetNamespace() || object.GetYtsaurusName() != c.GetName() {
		return fmt.Errorf("deleting unrelated component object: %v", object.GetName())
	}
	if oldObject, found := c.Objects[component]; found && oldObject.GetName() != object.GetName() {
		return fmt.Errorf("deleting component by different object: %v", object.GetName())
	}
	if _, found := c.Components[component]; found {
		return fmt.Errorf("deleting inline component: %v", component)
	}
	delete(c.Objects, object.GetComponentSpec().GetComponent())
	return nil
}

func (c *Cluster) FetchComponentObjects(ctx context.Context, reader client.Reader) error {
	lists := []client.ObjectList{
		&ytv1.YtsaurusUpdatePlanList{},
		&ytv1.YtsaurusExecNodesList{},
		&ytv1.YtsaurusDataNodesList{},
		&ytv1.YtsaurusTabletNodesList{},
		&ytv1.YtsaurusHTTPProxiesList{},
		&ytv1.YtsaurusRPCProxiesList{},
	}
	for _, list := range lists {
		if err := reader.List(ctx, list, client.InNamespace(c.Namespace)); err != nil {
			return err
		}
		items, err := meta.ExtractList(list)
		if err != nil {
			return err
		}
		for _, item := range items {
			object, ok := item.(ComponentObject)
			if !ok {
				return fmt.Errorf("invalid list object: %T", list)
			}
			if object.GetYtsaurusName() != c.Name {
				continue
			}
			err := c.AddComponentObject(object)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Cluster) LinkComponents() error {
	components := slices.SortedFunc(maps.Keys(c.Objects), func(a, b ytv1.Component) int {
		return cmp.Or(strings.Compare(string(a.Type), string(b.Type)), strings.Compare(a.Name, b.Name))
	})
	for _, component := range components {
		object := c.Objects[component]
		if object.GetNamespace() != c.GetNamespace() || object.GetYtsaurusName() != c.GetName() {
			return fmt.Errorf("unrelated object: %v", component)
		}
		if _, found := c.Components[component]; found {
			return fmt.Errorf("duplicate component: %v", component)
		}
		spec := object.GetComponentSpec()
		c.Components[component] = spec
		switch component.Type {
		case ytv1.ExecNodeType:
			s, ok := spec.(*ytv1.ExecNodesSpec)
			if !ok {
				return fmt.Errorf("unsupported spec: %v", component)
			}
			c.ExecNodes = append(c.ExecNodes, s)
		case ytv1.DataNodeType:
			s, ok := spec.(*ytv1.DataNodesSpec)
			if !ok {
				return fmt.Errorf("unsupported spec: %v", component)
			}
			c.DataNodes = append(c.DataNodes, s)
		case ytv1.TabletNodeType:
			s, ok := spec.(*ytv1.TabletNodesSpec)
			if !ok {
				return fmt.Errorf("unsupported spec: %v", component)
			}
			c.TabletNodes = append(c.TabletNodes, s)
		case ytv1.HttpProxyType:
			s, ok := spec.(*ytv1.HTTPProxiesSpec)
			if !ok {
				return fmt.Errorf("unsupported spec: %v", component)
			}
			c.HTTPProxies = append(c.HTTPProxies, s)
		case ytv1.RpcProxyType:
			s, ok := spec.(*ytv1.RPCProxiesSpec)
			if !ok {
				return fmt.Errorf("unsupported spec: %v", component)
			}
			c.RPCProxies = append(c.RPCProxies, s)
		case ytv1.UpdatePlanType:
			plan, ok := object.(*ytv1.YtsaurusUpdatePlan)
			if !ok {
				return fmt.Errorf("unsupported update plan: %v", component)
			}
			if c.UpdatePlanObject != nil {
				return fmt.Errorf("duplicate update plan")
			}
			c.UpdatePlanObject = plan
		default:
			return fmt.Errorf("unsupported component: %v", component)
		}
	}
	return nil
}

func (c *Cluster) GetComponentSpecs() []ytv1.ComponentSpec {
	components := c.Ytsaurus.GetComponentSpecs()
	for _, spec := range c.HTTPProxies[len(c.Spec.HTTPProxies):] {
		components = append(components, spec)
	}
	for _, spec := range c.RPCProxies[len(c.Spec.RPCProxies):] {
		components = append(components, spec)
	}
	for _, spec := range c.DataNodes[len(c.Spec.DataNodes):] {
		components = append(components, spec)
	}
	for _, spec := range c.ExecNodes[len(c.Spec.ExecNodes):] {
		components = append(components, spec)
	}
	for _, spec := range c.TabletNodes[len(c.Spec.TabletNodes):] {
		components = append(components, spec)
	}
	return components
}

func (c *Cluster) GetInstanceCount(component ytv1.Component) int {
	if spec, ok := c.Components[component]; ok {
		return spec.GetInstanceCount()
	}
	return 0
}

func (c *Cluster) GetUpdatePlan() ytv1.UpdatePlan {
	// Update plan is defined in spec inline.
	if len(c.Spec.UpdatePlan) != 0 {
		return c.Spec.UpdatePlan
	}
	// Update plan is defined in separate object.
	if c.UpdatePlanObject != nil && len(c.UpdatePlanObject.Spec.UpdatePlan) != 0 {
		return c.UpdatePlanObject.Spec.UpdatePlan
	}
	return []ytv1.ComponentUpdateSelector{{
		Class: ytv1.ComponentClassNothing,
	}}
}
