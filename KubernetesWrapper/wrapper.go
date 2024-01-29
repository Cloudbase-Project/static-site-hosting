package kuberneteswrapper

import (
	"context"
	"os"
	"regexp"
	"time"

	"github.com/Cloudbase-Project/static-site-hosting/constants"
	"github.com/Cloudbase-Project/static-site-hosting/utils"
	"k8s.io/client-go/kubernetes"

	// appsv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
)

type KubernetesWrapper struct {
	KClient *kubernetes.Clientset
}

type ImageBuilder struct {
	Ctx       context.Context
	Namespace string
	SiteId    string
	ImageName string
}

type DeploymentOptions struct {
	Ctx             context.Context
	Namespace       string
	SiteId          string
	DeploymentLabel map[string]string
	ImageName       string
	Replicas        int32
}

type ServiceOptions struct {
	Ctx             context.Context
	Namespace       string
	SiteId          string
	DeploymentLabel map[string]string
}

type UpdateOptions struct {
	Ctx       context.Context
	Namespace string
	Name      string
}

type DeleteOptions struct {
	Ctx       context.Context
	Name      string
	Namespace string
}

func NewWrapper(client *kubernetes.Clientset) *KubernetesWrapper {
	return &KubernetesWrapper{KClient: client}
}

func (kw *KubernetesWrapper) BuildLabel(key string, value []string) (*labels.Requirement, error) {
	return labels.NewRequirement(key, selection.Equals, value)
}

func (kw *KubernetesWrapper) GetImageBuilderWatcher(
	ctx context.Context,
	label string,
) (watch.Interface, error) {
	return kw.KClient.CoreV1().
		Pods(constants.Namespace).
		Watch(
			ctx,
			metav1.ListOptions{LabelSelector: label})
}

func (kw *KubernetesWrapper) GetDeploymentWatcher(
	ctx context.Context,
	label string,
	namespace string,
) (watch.Interface, error) {
	return kw.KClient.AppsV1().
		Deployments(namespace).
		Watch(ctx, metav1.ListOptions{LabelSelector: label})
}

// Build an image for the given siteId and image name
func (kw *KubernetesWrapper) CreateImageBuilder(ib *ImageBuilder) (*corev1.Pod, error) {

	var Dockerfile string
	Dockerfile = constants.Dockerfile

	m1 := regexp.MustCompile(`"`)
	dockerfile := m1.ReplaceAllString(Dockerfile, `\"`)

	REGISTRY := os.Getenv("REGISTRY")
	BASE64_CREDENTIALS := os.Getenv("BASE64_CREDENTIALS")

	pod, err := kw.KClient.CoreV1().Pods(ib.Namespace).Create(ib.Ctx, &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kaniko-worker",
			Labels: map[string]string{
				"builder": ib.SiteId, // the code id
			},
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "setup-kaniko",
				Image: "yauritux/busybox-curl",
				Command: []string{
					"/bin/sh",
					"-c",
					`wget -O /workspace/build.zip http://cloudbase-ssh-svc:4000/worker/queue/ && ls -lash && echo -e "` + dockerfile + `" >> /workspace/Dockerfile && echo -e && echo -e "{\"auths\":{\"` + REGISTRY + `\":{\"auth\": \"` + BASE64_CREDENTIALS + `\" }}}" > /kaniko/.docker/config.json`,
				},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "shared",
					MountPath: "/workspace",
				}, {
					Name:      "dockerconfig",
					MountPath: "/kaniko/.docker",
				}},
			}},
			Containers: []corev1.Container{{
				Name:  "kaniko-executor",
				Image: "gcr.io/kaniko-project/executor:latest",
				Args: []string{
					"--dockerfile=/workspace/Dockerfile",
					"--context=dir:///workspace",
					"--destination=" + ib.ImageName,
				},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "shared",
					MountPath: "/workspace",
				}, {
					Name:      "dockerconfig",
					MountPath: "/kaniko/.docker",
				}},
			}},
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes: []corev1.Volume{{
				Name: "shared", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			},
				{
					Name: "dockerconfig", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
			},
		},
	}, metav1.CreateOptions{})
	return pod, err
}

func (kw *KubernetesWrapper) CreateNamespace(
	ctx context.Context,
	namespace string,
) (*corev1.Namespace, error) {

	return kw.KClient.CoreV1().
		Namespaces().
		Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})

}

func (kw *KubernetesWrapper) CreateDeployment(options *DeploymentOptions) (*v1.Deployment, error) {
	return kw.KClient.AppsV1().
		Deployments(options.Namespace).
		Create(options.Ctx,
			&v1.Deployment{
				TypeMeta: metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"},
				ObjectMeta: metav1.ObjectMeta{
					Name:   options.SiteId,
					Labels: map[string]string{"app": options.SiteId},
				},
				Spec: v1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: options.DeploymentLabel,
					},
					Replicas: &options.Replicas, // TODO: Have to do more here
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: options.DeploymentLabel},
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyAlways,
							Containers: []corev1.Container{{
								Name:  options.SiteId,
								Image: options.ImageName, // "image name from db", // should be ghcr.io/projectname/siteId:latest
								Ports: []corev1.ContainerPort{{ContainerPort: 3000}},
							}},
							ImagePullSecrets: []corev1.LocalObjectReference{{Name: "regcred"}},
						},
					},
				},
			}, metav1.CreateOptions{})
}

func (kw *KubernetesWrapper) CreateService(options *ServiceOptions) (*corev1.Service, error) {

	serviceName := utils.BuildServiceName(options.SiteId)

	return kw.KClient.CoreV1().
		Services(options.Namespace).
		Create(options.Ctx, &corev1.Service{
			TypeMeta: metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{
				Name: serviceName,
			},
			Spec: corev1.ServiceSpec{
				Selector: options.DeploymentLabel,
				Type:     corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{Port: 4000, TargetPort: intstr.FromInt(4000)},
				},
			},
		}, metav1.CreateOptions{})
}

// Delete the deployment
func (kw *KubernetesWrapper) DeleteDeployment(options *DeleteOptions) error {
	return kw.KClient.AppsV1().
		Deployments(options.Namespace).
		Delete(options.Ctx, options.Name, metav1.DeleteOptions{})
}

// Delete the service
func (kw *KubernetesWrapper) DeleteService(options *DeleteOptions) error {
	return kw.KClient.CoreV1().
		Services(options.Namespace).
		Delete(options.Ctx, options.Name, metav1.DeleteOptions{})
}

// updates the deployment label with current timestamp to trigger a redeploy
func (kw *KubernetesWrapper) UpdateDeployment(options *UpdateOptions) error {

	deployment, err := kw.KClient.AppsV1().
		Deployments(options.Namespace).
		Get(options.Ctx, options.Name, metav1.GetOptions{})

	if err != nil {
		return err
	}

	deployment.Spec.Template.ObjectMeta.Annotations["date"] = time.Now().String()

	_, err = kw.KClient.AppsV1().
		Deployments(options.Namespace).
		Update(options.Ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}
