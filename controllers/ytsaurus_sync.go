package controllers

import (
	"context"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

// Sync2 handles single iteration of the Ytsaurus CRD reconciliation loop.
// Its does one simple thing: find one component which is not in desired state and ask it to reconcile itself.
// Currently, components dependency graph is linear, but it may be changed in the future.
func (r *YtsaurusReconciler) Sync2(ctx context.Context, resource *ytv1.Ytsaurus) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	var err error

	ytsaurus := apiProxy.NewYtsaurus(resource, r.Client, r.Recorder, r.Scheme)
	// TODO: can dependency graph contain not only components, but some kind of conditions
	// or it all can be done inside of the components?
	dependencyGraph, err := getDependencyGraph(ctx, ytsaurus)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	for _, component := range dependencyGraph {
		err = component.Fetch(ctx)
		if err != nil {
			logger.Error(err, "failed to fetch component", "component", component.GetName())
			return ctrl.Result{Requeue: true}, err
		}

		status := component.Status(ctx)
		switch status.SyncStatus {
		case components.SyncStatusReady:
			// This component is all good, proceed to the next one.
			continue
		case components.SyncStatusBlocked:
			// This component can't be updated by operator, human needed.
			// Not sure of semantics of this status. Can we understand that case at all?
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		case components.SyncStatusUpdating:
			// This component is already being updating, nothing to do for now.
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		case components.SyncStatusPending:
			// This component is not ready and operator should act here.
			// TODO: it is possible we will want to destroy/scale down all the components after that in reverse order.
			err = component.Sync2(ctx)
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
	}

	// Ytsaurus running and happy.
	return ctrl.Result{}, nil
}

var (
	// TODO: consts for types
	componentsOrder = []string{
		"Discovery",
		"Master",
		"HttpProxy",
		"YtsaurusClient",
		// Datanodes are the list of components — should we handle them separately?
		// I think so if it is separate sts.
	}
)

func getDependencyGraph(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) ([]components.SubComponent, error) {
	var comps []components.SubComponent

	cfgen := ytconfig.NewGenerator(ytsaurus.GetResource(), getClusterDomain(ytsaurus.APIProxy().Client()))
	comps = append(comps, components.NewMaster2(cfgen, ytsaurus))

	//componentManager, err := NewComponentManager(ctx, ytsaurus)
	//if err != nil {
	//	return comps, err
	//}
	//compsMap := make(map[string]components.SubComponent)
	//for _, comp := range componentManager.allComponents {
	//	// TODO: GetType() or smth
	//	compsMap[comp.GetName()] = comp
	//}
	// TODO: sort according to some componentsOrder
	return comps, nil
}
