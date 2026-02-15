/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/utils/ptr"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/validators"
)

// YtsaurusReconciler reconciles a Ytsaurus object
type YtsaurusReconciler struct {
	BaseReconciler
}

const configOverridesField = ".spec.configOverrides"

// +kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=ytsaurus,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=ytsaurus/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=ytsaurus/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulset,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulset/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=pod,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pod/status,verbs=get
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
func (r *YtsaurusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var ytsaurus ytv1.Ytsaurus
	if err := r.Get(ctx, req.NamespacedName, &ytsaurus); err != nil {
		logger.Error(err, "unable to fetch Ytsaurus")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !ptr.Deref(ytsaurus.Spec.IsManaged, true) || r.ShouldIgnoreResource(ctx, &ytsaurus) {
		logger.Info("Ytsaurus cluster is not managed by controller, do nothing")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	if err := validators.ValidateVersionConstraint(ytsaurus.Spec.RequiresOperatorVersion); err != nil {
		logger.Error(err, "Operator version does not satisfy spec version constraint")
		return ctrl.Result{}, err
	}

	return r.Sync(ctx, &ytsaurus)
}

// SetupWithManager sets up the controller with the Manager.
func (r *YtsaurusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// See https://book.kubebuilder.io/reference/watching-resources/externally-managed for the reference implementation
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &ytv1.Ytsaurus{}, configOverridesField, func(rawObj client.Object) []string {
		ytsaurusResource := rawObj.(*ytv1.Ytsaurus)
		if ytsaurusResource.Spec.ConfigOverrides == nil {
			return nil
		}
		return []string{ytsaurusResource.Spec.ConfigOverrides.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithLogConstructor(func(r *reconcile.Request) logr.Logger {
			log := mgr.GetLogger()
			if r != nil {
				log = log.WithValues("ytsaurus", r.NamespacedName.String())
			}
			return log
		}).
		For(&ytv1.Ytsaurus{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Secret{}).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForConfigMap),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

func (r *YtsaurusReconciler) findObjectsForConfigMap(ctx context.Context, configMap client.Object) []reconcile.Request {
	// See https://book.kubebuilder.io/reference/watching-resources/externally-managed for the reference implementation
	attachedYtsauruses := &ytv1.YtsaurusList{}
	listOps := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(configOverridesField, configMap.GetName()),
		Namespace:     configMap.GetNamespace(),
	}
	err := r.List(ctx, attachedYtsauruses, listOps)
	if err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(attachedYtsauruses.Items))
	for i, item := range attachedYtsauruses.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			},
		}
	}
	return requests
}

func (r *YtsaurusReconciler) Sync(ctx context.Context, resource *ytv1.Ytsaurus) (ctrl.Result, error) {
	logger := log.FromContext(ctx, "ytsaurusState", resource.Status.State)
	ctx = log.IntoContext(ctx, logger)

	ytsaurus := apiproxy.NewYtsaurus(resource, r.Client, r.Recorder, r.Scheme)
	cm, err := NewComponentManager(ctx, ytsaurus, r.ClusterDomain)
	if err != nil {
		logger.Error(err, "Cannot build component manager")
		return ctrl.Result{Requeue: true}, err
	}

	err = cm.FetchStatus(ctx)
	if err != nil {
		logger.Error(err, "Cannot fetch component manager status")
		return ctrl.Result{Requeue: true}, err
	}

	switch ytsaurus.GetClusterState() {
	case ytv1.ClusterStateCreated, "":
		logger.Info("Ytsaurus is just created and needs initialization")
		err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateInitializing)
		return ctrl.Result{Requeue: true}, err

	case ytv1.ClusterStateInitializing:
		// Ytsaurus has finished initializing, and is running now.
		if cm.status.allRunning {
			logger.Info("Ytsaurus has synced and is running now")
			err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateRunning)
			return ctrl.Result{Requeue: true}, err
		}

	case ytv1.ClusterStateRunning, ytv1.ClusterStateReconfiguration:
		// Apply current update plan and choose components to update.
		cm.applyUpdatePlan(resource.GetUpdatePlan())

		// All status updates _must_ be in one transaction with observed generation and new cluster state.
		needStatusUpdate := ytsaurus.SyncObservedGeneration()

		// There may be the case when some components needed update, but spec was reverted
		// and Updating never happen — so blocked components column need to be always actualized.
		if ytsaurus.SetBlockedComponents(cm.status.cannotUpdate) {
			needStatusUpdate = true
		}

		switch {
		case cm.status.allReady:
			logger.Info("Ytsaurus is running and happy")
			if ytsaurus.SetClusterState(ytv1.ClusterStateRunning) {
				needStatusUpdate = true
			}

		case !cm.status.allRunning:
			logger.Info("Ytsaurus needs initialization of some components")
			if ytsaurus.SetClusterState(ytv1.ClusterStateReconfiguration) {
				needStatusUpdate = true
			}

		case len(cm.status.needUpdate) == 0:
			logger.Info("All components are up-to-date")
			if ytsaurus.SetClusterState(ytv1.ClusterStateRunning) {
				needStatusUpdate = true
			}

		case len(cm.status.canUpdate) != 0:
			logger.Info("Ytsaurus components needs update",
				"canUpdate", cm.status.canUpdate,
				"cannotUpdate", cm.status.cannotUpdate,
			)
			// We do not update BlockedComponentsSummary here, it should be updated first thing in Running state.
			ytsaurus.SetUpdatingComponents(cm.status.canUpdate)
			ytsaurus.SetUpdateState(ytv1.UpdateStateNone)
			ytsaurus.SetClusterState(ytv1.ClusterStateUpdating)
			needStatusUpdate = true

		case len(cm.status.cannotUpdate) != 0:
			logger.Info("Ytsaurus components update is blocked",
				"cannotUpdate", cm.status.cannotUpdate,
			)
			// TODO: Add cluster state "UpdateBlocked".
			if ytsaurus.SetClusterState(ytv1.ClusterStateRunning) {
				needStatusUpdate = true
			}
		}

		// Have passed final check - save status update if needed.
		if needStatusUpdate {
			err := ytsaurus.UpdateStatus(ctx)
			return ctrl.Result{Requeue: true}, err
		}

		if ytsaurus.IsRunning() {
			// All done, nothing change - do not requeue reconcile.
			return ctrl.Result{}, nil
		}

	case ytv1.ClusterStateUpdating:
		cm.status.nowUpdating = ytsaurus.GetUpdatingComponents()

		logger.Info("Ytsaurus update",
			"updateState", ytsaurus.GetUpdateState(),
			"updatingComponents", cm.status.nowUpdating,
		)
		ytsaurus.RecordNormal("Update", fmt.Sprintf("Update flow starting with %s, updating components: %v", ytsaurus.GetUpdateState(), cm.status.nowUpdating))

		updateFlow := buildFlowTree(cm)
		progressed, err := updateFlow.execute(ctx, ytsaurus, cm)
		if err != nil {
			return ctrl.Result{}, err
		}

		if progressed {
			return ctrl.Result{Requeue: true}, err
		}

	case ytv1.ClusterStateCancelUpdate:
		if err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateNone); err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		if err := ytsaurus.ClearUpdateStatus(ctx); err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		logger.Info("Ytsaurus update was canceled, ytsaurus is running now")
		// We don't update observed generation because the update was not really finished,
		// and it's still the old version running.
		err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateRunning)
		return ctrl.Result{}, err

	case ytv1.ClusterStateUpdateFinishing:
		if err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateNone); err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		if err := ytsaurus.ClearUpdateStatus(ctx); err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		logger.Info("Ytsaurus update was finished and Ytsaurus is running now")
		err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateRunning)
		// Requeue once again to do final check and maybe update observed generation.
		return ctrl.Result{Requeue: true}, err

	default:
		return ctrl.Result{}, fmt.Errorf("unknown cluster state: %q", ytsaurus.GetClusterState())
	}

	return cm.Sync(ctx)
}
