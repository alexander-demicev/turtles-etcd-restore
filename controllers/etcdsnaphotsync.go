package controllers

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/util"
	capiutil "sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type EtcdSnapshotSyncReconciler struct {
	Client client.Client

	controller controller.Controller
	Tracker    *remote.ClusterCacheTracker
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
		return ctrl.Result{RequeueAfter: 3 * time.Minute}, nil
	}

	if !cluster.Status.ControlPlaneReady {
		log.Info("Control plane is not ready, skipping reconciliation")
		return ctrl.Result{RequeueAfter: 3 * time.Minute}, nil
	}

	log.Info("Setting up watch on ETCDSnapshotFile")

	etcdnapshotFile := &unstructured.Unstructured{}
	etcdnapshotFile.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k3s.cattle.io",
		Kind:    "ETCDSnapshotFile",
		Version: "v1",
	})

	if err := r.Tracker.Watch(ctx, remote.WatchInput{
		Name:         "cluster-watchConfigMapWithRestore",
		Cluster:      capiutil.ObjectKey(cluster),
		Watcher:      r.controller,
		Kind:         etcdnapshotFile,
		EventHandler: handler.EnqueueRequestsFromMapFunc(r.etcdSnapshotFile(ctx, cluster)),
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to start watch for nodes: %w", err)
	}

	remoteClient, err := r.Tracker.GetClient(ctx, capiutil.ObjectKey(cluster))
	if err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Listing etcd snapshot files")

	etcdnapshotFileList := &unstructured.UnstructuredList{}
	etcdnapshotFileList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k3s.cattle.io",
		Kind:    "ETCDSnapshotFile",
		Version: "v1",
	})

	if err := remoteClient.List(ctx, etcdnapshotFileList); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list etcd snapshot files: %w", err)
	}

	for _, snapshotFile := range etcdnapshotFileList.Items {
		log.Info("Found etcd snapshot file", "name", snapshotFile.GetName())

	}

	return ctrl.Result{}, nil
}

func (r *EtcdSnapshotSyncReconciler) etcdSnapshotFile(ctx context.Context, cluster *clusterv1.Cluster) handler.MapFunc {
	log := log.FromContext(ctx)

	return func(_ context.Context, o client.Object) []ctrl.Request {
		log.Info("Cluster name", "name", cluster.GetName())

		gvk := schema.GroupVersionKind{
			Group:   "k3s.cattle.io",
			Kind:    "ETCDSnapshotFile",
			Version: "v1",
		}

		if o.GetObjectKind().GroupVersionKind() != gvk {
			panic(fmt.Sprintf("Expected a %s but got a %s", gvk, o.GetObjectKind().GroupVersionKind()))
		}

		return []reconcile.Request{{NamespacedName: util.ObjectKey(cluster)}}
	}
}
