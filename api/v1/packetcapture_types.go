package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PacketCaptureSpec defines the desired state of PacketCapture
type PacketCaptureSpec struct {
	// TargetSelector определяет Pod, с которого нужно снять трафик.
	// Это будет либо имя/UID, либо набор меток (label selector).
	// K8s-операторы часто используют метки для гибкости.
	TargetSelector *metav1.LabelSelector `json:"targetSelector"`

	// DurationSeconds определяет, как долго должен длиться захват.
	// 0 или nil может означать ручную остановку.
	DurationSeconds int32 `json:"durationSeconds"`

	// FilterExpression - выражение в формате BPF (Berkeley Packet Filter),
	// например, "tcp port 80 and host 10.0.0.1".
	FilterExpression string `json:"filterExpression,omitempty"`
}

// PacketCaptureStatus defines the observed state of PacketCapture.
type PacketCaptureStatus struct {
	// Phase описывает текущее состояние: Pending, Running, Complete, Failed.
	Phase string `json:"phase"`

	// NodeName - имя ноды, на которой запущен Job-сниффер (для Этапа 3).
	NodeName string `json:"nodeName,omitempty"`

	// PcapFilename - имя конечного файла с дампом.
	PcapFilename string `json:"pcapFilename,omitempty"`

	// DownloadURL - URL для скачивания файла (для Этапа 5).
	DownloadURL string `json:"downloadURL,omitempty"`

	// StartTime - время начала захвата.
	StartTime *metav1.Time `json:"startTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PacketCapture is the Schema for the packetcaptures API
type PacketCapture struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of PacketCapture
	// +required
	Spec PacketCaptureSpec `json:"spec"`

	// status defines the observed state of PacketCapture
	// +optional
	Status PacketCaptureStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// PacketCaptureList contains a list of PacketCapture
type PacketCaptureList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []PacketCapture `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PacketCapture{}, &PacketCaptureList{})
}
