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
	// Ytsaurus is a main spec for cluster.
	*ytv1.Ytsaurus

	// Objects points to origin objects for separate components.
	Objects map[ytv1.Component]ComponentObject

	// Components contains all components - defined inline and separately.
	Components map[ytv1.Component]ytv1.ComponentSpec

	// Following slices contains inline components first, in the order of definition;
	// and then separately defined components in stable alphabetically sorted order.

	ExecNodes   []*ytv1.ExecNodesSpec
	DataNodes   []*ytv1.DataNodesSpec
	TabletNodes []*ytv1.TabletNodesSpec
	HTTPProxies []*ytv1.HTTPProxiesSpec
	RPCProxies  []*ytv1.RPCProxiesSpec
}

// ComponentObject must be implemented by every object which defines component separately.
type ComponentObject interface {
	client.Object

	GetYtsaurusName() string
	GetStatusObservedGeneration() int64
	SetStatusObservedGeneration(generation int64)
	GetComponentSpec() ytv1.ComponentSpec
	GetComponentStatus() ytv1.ComponentStatus
	SetComponentStatus(status ytv1.ComponentStatus)
}

// KnownYtsaurusComponentObjects defines component object types watched by Ytsaurus reconciler.
func KnownYtsaurusComponentObjects() []ComponentObject {
	return []ComponentObject{}
}

// knownYtsaurusComponentObjectLists defines component object lists fetched by FetchComponentObjects.
func knownYtsaurusComponentObjectLists() []client.ObjectList {
	return []client.ObjectList{}
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

// componentGenerationKey is a key for tracking generations of separated objects.
func componentGenerationKey(component ytv1.Component, object ComponentObject) string {
	return component.String() + "@" + string(object.GetUID())
}

// AreComponentGenerationsObserved returns true if generation of cluster spec and all separated objects are observed.
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

// UpdateObservedComponentGenerations saved current generations of separated objects as observed.
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
	knownLists := knownYtsaurusComponentObjectLists()
	for _, list := range knownLists {
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

func appendComponentSpec[T ytv1.ComponentSpec](list *[]T, spec ytv1.ComponentSpec) error {
	s, ok := spec.(T)
	if !ok {
		return fmt.Errorf("unsupported spec type: %v", spec.GetComponent())
	}
	*list = append(*list, s)
	return nil
}

// LinkComponents incorporates specs for separately defined cluster components.
func (c *Cluster) LinkComponents() error {
	// Link separated components in order sorted by type and name.
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
		var err error
		switch component.Type {
		case ytv1.ExecNodeType:
			err = appendComponentSpec(&c.ExecNodes, spec)
		case ytv1.DataNodeType:
			err = appendComponentSpec(&c.DataNodes, spec)
		case ytv1.TabletNodeType:
			err = appendComponentSpec(&c.TabletNodes, spec)
		case ytv1.HttpProxyType:
			err = appendComponentSpec(&c.HTTPProxies, spec)
		case ytv1.RpcProxyType:
			err = appendComponentSpec(&c.RPCProxies, spec)
		default:
			err = fmt.Errorf("unsupported component spec: %v", component)
		}
		if err != nil {
			return err
		}
		c.Components[component] = spec
	}
	return nil
}

// GetComponentSpecs returns specs for all components declared in Ytsaurus and in separate objects.
func (c *Cluster) GetComponentSpecs() []ytv1.ComponentSpec {
	components := c.Ytsaurus.GetComponentSpecs()
	// Separately defined components are comes after components declared inline.
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

// GetInstanceCount returns defined instance count and -1 for undefined components.
// NOTE: "for i := range -1" does 0 iterations.
func (c *Cluster) GetInstanceCount(component ytv1.Component) int {
	if spec, ok := c.Components[component]; ok {
		return spec.GetInstanceCount()
	}
	return -1
}
