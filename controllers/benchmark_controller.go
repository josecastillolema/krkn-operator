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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	perfv1 "github.com/josecastillolema/krkn-operator/api/v1"
)

const image = "quay.io/redhat-chaos/krkn-hub"

// Definitions to manage status conditions
const (
	// typeAvailableBenchmark represents the status of the Deployment reconciliation
	typeAvailableBenchmark = "Available"
	// typeDegradedBenchmark represents the status used when the custom resource is deleted and the finalizer operations are must to occur.
	typeDegradedBenchmark = "Degraded"
)

// BenchmarkReconciler reconciles a Benchmark object
type BenchmarkReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// The following markers are used to generate the rules permissions (RBAC) on config/rbac using controller-gen
// when the command <make manifests> is executed.
// To know more about markers see: https://book.kubebuilder.io/reference/markers.html

//+kubebuilder:rbac:groups=perf.chaos.io,resources=benchmarks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=perf.chaos.io,resources=benchmarks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=perf.chaos.io,resources=benchmarks/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

// It is essential for the controller's reconciliation loop to be idempotent. By following the Operator
// pattern you will create Controllers which provide a reconcile function
// responsible for synchronizing resources until the desired state is reached on the cluster.
// Breaking this recommendation goes against the design principles of controller-runtime.
// and may lead to unforeseen consequences such as resources becoming stuck and requiring manual intervention.
// For further info:
// - About Operator Pattern: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/
// - About Controllers: https://kubernetes.io/docs/concepts/architecture/controller/
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *BenchmarkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Benchmark instance
	// The purpose is check if the Custom Resource for the Benchmark kind
	// is applied on the cluster if not we return nil to stop the reconciliation
	benchmark := &perfv1.Benchmark{}
	err := r.Get(ctx, req.NamespacedName, benchmark)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("Benchmark resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Benchmark")
		return ctrl.Result{}, err
	}

	// Let's just set the status as Unknown when no status are available
	if benchmark.Status.Conditions == nil || len(benchmark.Status.Conditions) == 0 {
		meta.SetStatusCondition(&benchmark.Status.Conditions, metav1.Condition{Type: typeAvailableBenchmark, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err = r.Status().Update(ctx, benchmark); err != nil {
			log.Error(err, "Failed to update Benchmark status")
			return ctrl.Result{}, err
		}

		// Let's re-fetch the bechmark Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		if err := r.Get(ctx, req.NamespacedName, benchmark); err != nil {
			log.Error(err, "Failed to re-fetch benchmark")
			return ctrl.Result{}, err
		}
	}

	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: benchmark.Name, Namespace: benchmark.Namespace}, found)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new deployment
		dep, err := r.deploymentForBenchmark(benchmark)
		if err != nil {
			log.Error(err, "Failed to define new Deployment resource for Benchmark")

			// The following implementation will update the status
			meta.SetStatusCondition(&benchmark.Status.Conditions, metav1.Condition{Type: typeAvailableBenchmark,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create Deployment for the custom resource (%s): (%s)", benchmark.Name, err)})

			if err := r.Status().Update(ctx, benchmark); err != nil {
				log.Error(err, "Failed to update Benchmark status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		log.Info("Creating a new Deployment",
			"Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		if err = r.Create(ctx, dep); err != nil {
			log.Error(err, "Failed to create new Deployment",
				"Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}

		// Deployment created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	// The following implementation will update the status
	meta.SetStatusCondition(&benchmark.Status.Conditions, metav1.Condition{Type: typeAvailableBenchmark,
		Status: metav1.ConditionTrue, Reason: "Reconciling",
		Message: fmt.Sprintf("Deployment for custom resource (%s) created successfully", benchmark.Name)})

	if err := r.Status().Update(ctx, benchmark); err != nil {
		log.Error(err, "Failed to update Benchmark status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// deploymentForBenchmark returns a Benchmark Deployment object
func (r *BenchmarkReconciler) deploymentForBenchmark(
	benchmark *perfv1.Benchmark) (*appsv1.Deployment, error) {
	ls := labelsForBenchmark(benchmark.Name)
	replicas := int32(1)

	// Get the Operand image
	image := image + ":" + benchmark.Spec.Scenario

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      benchmark.Name,
			Namespace: benchmark.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					// TODO(user): Uncomment the following code to configure the nodeAffinity expression
					// according to the platforms which are supported by your solution. It is considered
					// best practice to support multiple architectures. build your manager image using the
					// makefile target docker-buildx. Also, you can use docker manifest inspect <image>
					// to check what are the platforms supported.
					// More info: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity
					//Affinity: &corev1.Affinity{
					//	NodeAffinity: &corev1.NodeAffinity{
					//		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					//			NodeSelectorTerms: []corev1.NodeSelectorTerm{
					//				{
					//					MatchExpressions: []corev1.NodeSelectorRequirement{
					//						{
					//							Key:      "kubernetes.io/arch",
					//							Operator: "In",
					//							Values:   []string{"amd64", "arm64", "ppc64le", "s390x"},
					//						},
					//						{
					//							Key:      "kubernetes.io/os",
					//							Operator: "In",
					//							Values:   []string{"linux"},
					//						},
					//					},
					//				},
					//			},
					//		},
					//	},
					//},
					Containers: []corev1.Container{{
						Image: image,
						Name:  "benchmark",
						Env: []corev1.EnvVar{
							{
								Name:  "MYSQL_ROOT_PASSWORD",
								Value: "olar2",
							},
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
						SecurityContext: &corev1.SecurityContext{
							Privileged: &[]bool{true}[0],
						},
					}},
				},
			},
		},
	}

	// Set the ownerRef for the Deployment
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(benchmark, dep, r.Scheme); err != nil {
		return nil, err
	}
	return dep, nil
}

// labelsForBechmark returns the labels for selecting the resources
// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
func labelsForBenchmark(name string) map[string]string {
	return map[string]string{"app.kubernetes.io/name": "Benchmark",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/part-of":    "krkn-operator",
		"app.kubernetes.io/created-by": "controller-manager",
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *BenchmarkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&perfv1.Benchmark{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
