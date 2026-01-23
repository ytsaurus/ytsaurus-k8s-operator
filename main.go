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

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap/zapcore"

	"github.com/ytsaurus/ytsaurus-k8s-operator/controllers"
	"github.com/ytsaurus/ytsaurus-k8s-operator/validators"

	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/version"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(ytv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var watchOperatorInstance string
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var maxConcurrentReconciles int
	flag.BoolFunc("version", "print the version", func(s string) error {
		version.PrintVersion()
		os.Exit(0)
		return nil
	})
	flag.StringVar(&watchOperatorInstance, "watch-operator-instance", os.Getenv("WATCH_OPERATOR_INSTANCE"),
		fmt.Sprintf("Restricts reconciler scope to resources with matching label %q", consts.YTOperatorInstanceLabelName))
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&maxConcurrentReconciles, "max-concurrent-reconciles", 1, "The maximum number of concurrent Reconciles which can be run")
	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	enableWebhooks := os.Getenv("ENABLE_WEBHOOKS") != "false"

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	ctx := ctrl.SetupSignalHandler()

	var clusterDomain string
	if domain := os.Getenv("K8S_CLUSTER_DOMAIN"); domain != "" {
		clusterDomain = domain
	} else if domain, err := controllers.GuessClusterDomain(ctx); err == nil {
		clusterDomain = domain
	} else {
		setupLog.Error(err, "unable to guess cluster domain")
		os.Exit(1)
	}

	setupLog.Info(
		"Starting ytsaurus operator",
		"version", version.GetVersion(),
		"operatorInstance", watchOperatorInstance,
		"clusterDomain", clusterDomain,
	)

	managerOptions := ctrl.Options{
		Scheme: scheme,
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 9443,
		}),
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "6ab077f0.ytsaurus.tech",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
		Controller: controllerconfig.Controller{
			MaxConcurrentReconciles: maxConcurrentReconciles,
		},
	}

	// We can't setup managerOptions.Cache.DefaultNamespaces = map[cache.AllNamespaces]cache.Config{} due to
	// https://github.com/kubernetes-sigs/controller-runtime/issues/2628
	if watchNamespace := os.Getenv("WATCH_NAMESPACE"); watchNamespace != "" {
		managerOptions.Cache.DefaultNamespaces = map[string]cache.Config{}
		for _, namespace := range strings.Split(watchNamespace, ",") {
			managerOptions.Cache.DefaultNamespaces[namespace] = cache.Config{}
			setupLog.Info("Watching namespace", "namespace", namespace)
		}
		if len(managerOptions.Cache.DefaultNamespaces) == 1 {
			managerOptions.LeaderElectionNamespace = watchNamespace
		}
	}

	if watchOperatorInstance != "" {
		managerOptions.LeaderElectionID += "--" + watchOperatorInstance

		matchOperatorInstance, err := labels.NewRequirement(
			consts.YTOperatorInstanceLabelName,
			selection.Equals,
			[]string{watchOperatorInstance},
		)
		if err != nil {
			setupLog.Error(err, "unable to construct operator instance selector")
			os.Exit(1)
		}
		labelSelector := labels.NewSelector().Add(*matchOperatorInstance)

		managerOptions.Cache.ByObject = map[client.Object]cache.ByObject{}
		for _, object := range ytv1.KnownObjectTypes() {
			gvk, err := apiutil.GVKForObject(object, scheme)
			if err != nil {
				setupLog.Error(err, "unable to construct operator instance selector")
				os.Exit(1)
			}
			setupLog.Info("Cache label selector", "GVK", gvk, "selector", labelSelector)
			managerOptions.Cache.ByObject[object.(client.Object)] = cache.ByObject{
				Label: labelSelector,
			}
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), managerOptions)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	baseReconciler := func(name string) controllers.BaseReconciler {
		return controllers.BaseReconciler{
			WatchOperatorInstance: watchOperatorInstance,
			ClusterDomain:         clusterDomain,
			Client:                mgr.GetClient(),
			Scheme:                mgr.GetScheme(),
			Recorder:              mgr.GetEventRecorderFor(name),
		}
	}

	if err = (&controllers.YtsaurusReconciler{
		BaseReconciler: baseReconciler("ytsaurus-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Ytsaurus")
		os.Exit(1)
	}

	if enableWebhooks {
		if err = validators.NewYtsaurusValidator().SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Ytsaurus")
			os.Exit(1)
		}
	}

	if err = (&controllers.SpytReconciler{
		BaseReconciler: baseReconciler("spyt-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Spyt")
		os.Exit(1)
	}
	if enableWebhooks {
		if err = validators.NewSpytValidator().SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Spyt")
			os.Exit(1)
		}
	}

	if err = (&controllers.ChytReconciler{
		BaseReconciler: baseReconciler("chyt-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Chyt")
		os.Exit(1)
	}
	if enableWebhooks {
		if err = validators.NewChytValidator().SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Chyt")
			os.Exit(1)
		}
	}

	if err = (&controllers.RemoteExecNodesReconciler{
		BaseReconciler: baseReconciler("remoteexecnodes-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RemoteExecNodes")
		os.Exit(1)
	}

	if err = (&controllers.RemoteDataNodesReconciler{
		BaseReconciler: baseReconciler("remotedatanodes-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RemoteDataNodes")
		os.Exit(1)
	}

	if err = (&controllers.RemoteTabletNodesReconciler{
		BaseReconciler: baseReconciler("remotetabletnodes-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RemoteTabletNodes")
		os.Exit(1)
	}

	if err = (&controllers.OffshoreDataGatewaysReconciler{
		BaseReconciler: baseReconciler("offshoredatagateways-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OffshoreDataGateways")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
