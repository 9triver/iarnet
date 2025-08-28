package runner

import (
	"context"
	"fmt"

	"github.com/yourusername/container-peer-service/internal/resource"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type K8sRunner struct {
	clientset *kubernetes.Clientset
	namespace string
}

func NewK8sRunner() (*K8sRunner, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			return nil, err
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &K8sRunner{clientset: clientset, namespace: "default"}, nil
}

func (r *K8sRunner) Run(ctx context.Context, spec ContainerSpec) error {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "cps-pod-",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    "container",
					Image:   spec.Image,
					Command: spec.Command,
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%f", spec.CPU)),
							v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%fGi", spec.Memory)),
							"nvidia.com/gpu":  resource.MustParse(fmt.Sprintf("%f", spec.GPU)),
						},
						Limits: v1.ResourceList{ // Same as requests for simplicity
							v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%f", spec.CPU)),
							v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%fGi", spec.Memory)),
							"nvidia.com/gpu":  resource.MustParse(fmt.Sprintf("%f", spec.GPU)),
						},
					},
				},
			},
		},
	}
	_, err := r.clientset.CoreV1().Pods(r.namespace).Create(ctx, pod, metav1.CreateOptions{})
	return err
}

func (r *K8sRunner) Stop(podName string) error {
	return r.clientset.CoreV1().Pods(r.namespace).Delete(context.Background(), podName, metav1.DeleteOptions{})
}

func (r *K8sRunner) GetUsage() resource.ResourceUsage {
	// TODO: List pods with label, sum requests.
	return resource.ResourceUsage{} // Placeholder
}
