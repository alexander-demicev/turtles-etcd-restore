package controllers

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/controllers/remote"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type EtcdSnapshotSyncReconciler struct {
	Client client.Client

	controller         controller.Controller
	externalTracker    external.ObjectTracker
	remoteClientGetter remote.ClusterClientGetter
}

func (r *EtcdSnapshotSyncReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	// TODO: Setup predicates for the controller.
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).
		Build(r)
	if err != nil {
		return fmt.Errorf("creating new controller: %w", err)
	}

	r.controller = c
	r.externalTracker = external.ObjectTracker{
		Controller: c,
	}

	return nil
}

func (r *EtcdSnapshotSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, reterr error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling CAPI cluster")

	cluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{Requeue: true}, err
	}

	// Only reconcile RKE2 clusters
	if cluster.Spec.ControlPlaneRef.Kind != "RKE2ControlPlane" { // TODO: Move to predicate
		log.Info("Cluster is not an RKE2 cluster, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	log.Info("Setting up watch on config map")
	if err := r.externalTracker.Watch(ctx, remote.WatchInput{
		Name:         "cluster-watchConfigMapWithRestore",
		Cluster:      capiutil.ObjectKey(cluster),
		Watcher:      r.controller,
		Kind:         &corev1.ConfigMap{},
		EventHandler: handler.EnqueueRequestsFromMapFunc(r.nodeToMachine),
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to start watch for nodes: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *EtcdSnapshotSyncReconciler) nodeToMachine(ctx context.Context, o client.Object) []reconcile.Request {
	cm, ok := o.(*corev1.ConfigMap)
	if !ok {
		panic(fmt.Sprintf("Expected a ConfigMap but got a %T", o))
	}

}
