package controller

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	capturev1 "github.com/bibizyann/pod-sniffer/api/v1"
)

// PacketCaptureReconciler reconciles a PacketCapture object
type PacketCaptureReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=capture.io.github.bibizyann,resources=packetcaptures,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=capture.io.github.bibizyann,resources=packetcaptures/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=capture.io.github.bibizyann,resources=packetcaptures/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
func (r *PacketCaptureReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	packetCapture := &capturev1.PacketCapture{}
	if err := r.Get(ctx, req.NamespacedName, packetCapture); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger := log.FromContext(ctx)

	logger.V(1).Info("Получен заказ на дамп",
		"Имя CR", req.NamespacedName,
		"TargetSelector", packetCapture.Spec.TargetSelector)

	podList := &corev1.PodList{}
	selector, err := metav1.LabelSelectorAsSelector(packetCapture.Spec.TargetSelector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get labels: %w", err)
	}

	listOptions := []client.ListOption{
		client.InNamespace(packetCapture.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	}

	if err = r.List(ctx, podList, listOptions...); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get pod: %w", err)
	}

	if len(podList.Items) == 0 {
		logger.Info("Целевые поды не найдены, ждем...")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	targetPod := podList.Items[0]
	nodeName := targetPod.Spec.NodeName
	if nodeName == "" {
		logger.Info("Под еще не назначен на ноду, ждем...")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// TODO: решить что делать с дубликатами сниффера
	snifferJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      packetCapture.Name + "-sniffer",
			Namespace: packetCapture.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{
						"kubernetes.io/hostname": nodeName,
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						// TODO: add real image and command
						{
							Name:    "sniffer",
							Image:   "nginx:latest",
							Command: []string{"sleep", "3600"},
						},
					},
				},
			},
		},
	}

	ctrl.SetControllerReference(packetCapture, snifferJob, r.Scheme)

	if err := r.Create(ctx, snifferJob); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create sniffer-job: %w", err)
	}

	logger.Info("Successefully created pod-sniffer: %s", snifferJob.ObjectMeta.Name)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PacketCaptureReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capturev1.PacketCapture{}).
		Named("packetcapture").
		Complete(r)
}

func findNode(ctx context.Context, pod *corev1.Pod) *corev1.Node {

	return &corev1.Node{}
}
