package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	capturev1 "github.com/bibizyann/pod-sniffer/api/v1"
	"github.com/bibizyann/pod-sniffer/internal/storage"
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
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
func (r *PacketCaptureReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	packetCapture := &capturev1.PacketCapture{}
	if err := r.Get(ctx, req.NamespacedName, packetCapture); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if packetCapture.Status.Phase == "Completed" {
		return ctrl.Result{}, nil
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

	snifferJob := &batchv1.Job{}
	err = r.Get(ctx, types.NamespacedName{Name: packetCapture.Name + "-sniffer", Namespace: packetCapture.Namespace}, snifferJob)

	if err != nil && errors.IsNotFound(err) {
		fullID := targetPod.Status.ContainerStatuses[0].ContainerID
		containerID := strings.TrimPrefix(fullID, "containerd://")

		snifferJob = createSnifferJob(packetCapture, nodeName, containerID)

		if err := ctrl.SetControllerReference(packetCapture, snifferJob, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.Create(ctx, snifferJob); err != nil {
			return ctrl.Result{RequeueAfter: 10 * time.Second}, fmt.Errorf("failed to create sniffer-job: %w", err)
		}

		logger.Info("Successfully created pod-sniffer", "jobName", snifferJob.Name)

		return ctrl.Result{}, nil

	} else if err != nil {
		logger.Error(err, "failed to get Job")
		return ctrl.Result{}, err
	}

	secretKeys := &corev1.Secret{}
	err = r.Get(ctx, client.ObjectKey{Namespace: "default", Name: "yc-s3-keys"}, secretKeys)
	if err != nil {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, fmt.Errorf("failed to get secret: %w", err)
	}

	accessKey := string(secretKeys.Data["access-key"])
	secretKey := string(secretKeys.Data["secret-key"])

	sdk, err := storage.NewS3Storage(accessKey, secretKey, "pcap")
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to build sdk: %w", err)
	}

	if snifferJob.Status.Succeeded > 0 {
		if packetCapture.Status.Phase == "Completed" && packetCapture.Status.DownloadURL != "" {
			return ctrl.Result{}, nil
		}

		logger.Info("Job успешно завершена, генерируем ссылку")

		url, err := sdk.GeneratePresignedURL(ctx, "dump.pcap")
		if err != nil {
			return ctrl.Result{RequeueAfter: 10 * time.Second}, fmt.Errorf("failed to generate presigned URL: %w", err)
		}

		packetCapture.Status.Phase = "Completed"
		packetCapture.Status.DownloadURL = url
		if err := r.Status().Update(ctx, packetCapture); err != nil {
			return ctrl.Result{}, err
		}

		logger.Info("URL is ready", "URL", url)
		return ctrl.Result{}, nil
	}

	if snifferJob.Status.Failed > 0 {
		logger.Info("Job завершилась с ошибкой")
		packetCapture.Status.Phase = "Failed"
		r.Status().Update(ctx, packetCapture)
		return ctrl.Result{}, nil
	}

	if snifferJob.Status.Active > 0 {
		logger.Info("Захват в процессе...")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PacketCaptureReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capturev1.PacketCapture{}).
		Owns(&batchv1.Job{}).
		Named("packetcapture").
		Complete(r)
}

func createSnifferJob(packetCapture *capturev1.PacketCapture, nodeName string, containerID string) *batchv1.Job {
	snifferJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      packetCapture.Name + "-sniffer",
			Namespace: packetCapture.Namespace,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: ptr.To(int32(600)),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					HostPID: true,
					NodeSelector: map[string]string{
						"kubernetes.io/hostname": nodeName,
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "sniffer",
							Image:           "sniffer:latest",
							SecurityContext: &corev1.SecurityContext{Privileged: ptr.To(true)},
							Env: []corev1.EnvVar{
								{Name: "CONTAINER_ID", Value: containerID},
								{Name: "DURATION", Value: fmt.Sprint(packetCapture.Spec.DurationSeconds)},
								{Name: "FILTER", Value: packetCapture.Spec.FilterExpression},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "proc", MountPath: "/host/proc"},
								{Name: "runtime", MountPath: "/run/containerd/containerd.sock"},
								{Name: "data", MountPath: "/data"},
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
						{
							Name:            "uploader",
							Image:           "uploader:latest",
							ImagePullPolicy: corev1.PullIfNotPresent,
							VolumeMounts: []corev1.VolumeMount{
								{Name: "data", MountPath: "/data", ReadOnly: true},
							},
							Env: []corev1.EnvVar{
								{Name: "BUCKET_NAME", Value: "pcap"},
								{Name: "FILE_NAME", Value: "dump.pcap"},
								{Name: "AWS_DEFAULT_REGION", Value: "ru-central1"},
								{
									Name: "AWS_ACCESS_KEY_ID",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: "yc-s3-keys"},
											Key:                  "access-key",
										},
									},
								},
								{
									Name: "AWS_SECRET_ACCESS_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: "yc-s3-keys"},
											Key:                  "secret-key",
										},
									},
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{Name: "proc", VolumeSource: corev1.VolumeSource{HostPath: ptr.To(corev1.HostPathVolumeSource{Path: "/proc"})}},
						{Name: "runtime", VolumeSource: corev1.VolumeSource{HostPath: ptr.To(corev1.HostPathVolumeSource{Path: "/run/containerd/containerd.sock"})}},
						{Name: "data", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
					},
				},
			},
		},
	}

	return snifferJob
}
