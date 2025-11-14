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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

// SpytReconciler reconciles a Spyt object
type SpytReconciler struct {
	BaseReconciler
}

//+kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=spyts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=spyts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=spyts/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Spyt object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *SpytReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var spyt ytv1.Spyt
	if err := r.Get(ctx, req.NamespacedName, &spyt); err != nil {
		logger.Error(err, "unable to fetch Spyt")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var ytsaurus ytv1.Ytsaurus
	ytsaurusName := types.NamespacedName{Name: spyt.Spec.Ytsaurus.Name, Namespace: req.Namespace}
	if err := r.Get(ctx, ytsaurusName, &ytsaurus); err != nil {
		logger.Error(err, "unable to fetch Ytsaurus for spyt")
		return ctrl.Result{RequeueAfter: time.Second * 10}, err
	}

	logger.V(1).Info("found Spyt")

	return r.Sync(ctx, &spyt, &ytsaurus)
}

// SetupWithManager sets up the controller with the Manager.
func (r *SpytReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ytv1.Spyt{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
