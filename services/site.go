package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	kuberneteswrapper "github.com/Cloudbase-Project/static-site-hosting/KubernetesWrapper"
	"github.com/Cloudbase-Project/static-site-hosting/constants"
	"github.com/Cloudbase-Project/static-site-hosting/models"
	"gorm.io/gorm"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type SiteService struct {
	db *gorm.DB
	l  *log.Logger
}

type WatchResult struct {
	Status string
	Reason string
	Err    error
}

func NewSiteService(db *gorm.DB, l *log.Logger) *SiteService {
	return &SiteService{db: db, l: l}
}

func (fs *SiteService) GetAllSites(
	ownerId string,
	projectId string,
) (*models.Sites, error) {

	var sites models.Sites
	var config models.Config
	result := fs.db.Where(&models.Config{Owner: ownerId, ProjectId: projectId}).First(&config)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, errors.New("Invalid projectId")
	}
	if !config.Enabled {
		return nil, errors.New("static-site-hosting is disabled")
	}

	if err := fs.db.Where(&models.Site{ConfigID: config.ID}).Find(&sites).Error; err != nil {
		return nil, err
	}

	return &sites, nil
}

func (fs *SiteService) GetSite(
	siteId string,
	ownerId string,
	projectId string,
) (*models.Site, error) {
	var site models.Site
	var config models.Config

	result := fs.db.Where(&models.Config{Owner: ownerId, ProjectId: projectId}).First(&config)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, errors.New("Invalid projectId")
	}
	if !config.Enabled {
		return nil, errors.New("static-site-hosting is disabled")
	}

	if err := fs.db.First(&site, "id = ?", siteId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		} else {
			return nil, err
		}
	}
	return &site, nil
}

// Create a site in the db.
func (fs *SiteService) CreateSite(
	ownerId string,
	projectId string,
) (*models.Site, error) {

	var config models.Config

	var configs []models.Config

	fs.db.Find(&configs)
	fmt.Printf("configs.owner: %v\n", configs[0].Owner)
	fmt.Printf("configs.projectid: %v\n", configs[0].ProjectId)

	result := fs.db.Where(&models.Config{Owner: ownerId, ProjectId: projectId}).First(&config)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, errors.New("Invalid projectId")
	}
	if !config.Enabled {
		return nil, errors.New("static-site-hosting is disabled")
	}

	site := models.Site{Config: config}

	fs.db.Create(&site)

	fmt.Printf("result: %v\n", &result)
	return &site, nil
}

func (fs *SiteService) SaveSite(site *models.Site) {
	fs.db.Save(site)
}

// Delete a site with its primary key.
func (fs *SiteService) DeleteSite(siteId string, ownerId string, projectId string) error {

	var config models.Config

	result := fs.db.Where(&models.Config{Owner: ownerId, ProjectId: projectId}).First(&config)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return errors.New("Invalid projectId")
	}
	if !config.Enabled {
		return errors.New("static-site-hosting is disabled")
	}

	if err := fs.db.Where("id = ?", siteId).Delete(&models.Site{}).Error; err != nil {
		return err
	}
	return nil
}

// Deploys a site. Creates a deployment and a clusterIP service
func (fs *SiteService) DeploySite(
	kw *kuberneteswrapper.KubernetesWrapper,
	ctx context.Context,
	namespace string,
	siteId string,
	label map[string]string,
	imageName string,
	replicas int32,
) error {
	// (ctx, funtionid, namespace, imagename, replicas, label)

	_, err := kw.CreateDeployment(&kuberneteswrapper.DeploymentOptions{
		Ctx:             ctx,
		Namespace:       namespace,
		SiteId:          siteId,
		DeploymentLabel: label,
		ImageName:       imageName,
		Replicas:        replicas,
	})
	if err != nil {
		return err
	}

	// create a clusterIP service for the deployment
	_, err = kw.CreateService(&kuberneteswrapper.ServiceOptions{
		Ctx:             ctx,
		Namespace:       namespace,
		SiteId:          siteId,
		DeploymentLabel: label,
	})
	if err != nil {
		return err
	}
	return nil
}

func (fs *SiteService) WatchDeployment(
	kw *kuberneteswrapper.KubernetesWrapper,
	site *models.Site,
	namespace string,
) WatchResult {
	watchContext, cancelFunc := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFunc()

	label, _ := kw.BuildLabel("app", []string{site.ID.String()}) // TODO:
	deploymentWatch, err := kw.GetDeploymentWatcher(
		watchContext,
		label.String(),
		namespace,
	)
	if err != nil {
		return WatchResult{Err: err}
	}

	dataChan := make(chan WatchResult)

	go func(dataChan chan WatchResult) {
		for event := range deploymentWatch.ResultChan() {
			p, ok := event.Object.(*appsv1.Deployment)
			if !ok {
				fmt.Println("unexpected type")
				continue
			}
			fmt.Printf("p: %v\n", p)
			if p.Status.UpdatedReplicas == *p.Spec.Replicas &&
				p.Status.Replicas == *p.Spec.Replicas &&
				p.Status.AvailableReplicas == *p.Spec.Replicas &&
				p.Status.ObservedGeneration >= p.GetObjectMeta().GetGeneration() {
				// deployment complete
				fs.l.Print("Deployment available replicas = required replicas")
				if p.Status.Conditions[0].Type == appsv1.DeploymentAvailable {
					fs.l.Print("Deployment Available")
				}
				dataChan <- WatchResult{Status: string(constants.Deployed), Err: nil}
				deploymentWatch.Stop()
				break
			} else if p.Status.Conditions[0].Type == appsv1.DeploymentProgressing {
				fs.l.Print("Deployment in Progress")
			} else if p.Status.Conditions[0].Type == appsv1.DeploymentReplicaFailure {
				fs.l.Print("Replica failure. Reason : ", p.Status.Conditions[0].Message)
				dataChan <- WatchResult{Status: string(constants.DeploymentFailed), Reason: p.Status.Conditions[0].Message, Err: nil}
				fs.SaveSite(site)
				deploymentWatch.Stop()
				break

			}

		}
	}(dataChan)

	select {
	case <-watchContext.Done():
		return WatchResult{
			Status: string(constants.DeploymentFailed),
			Reason: "Watch Timeout",
			Err:    nil,
		}
	case x := <-dataChan:
		return x
	}
}

func (fs *SiteService) WatchImageBuilder(
	kw *kuberneteswrapper.KubernetesWrapper,
	site *models.Site,
	namespace string,
) WatchResult {

	// watch for 1 min and then close everything
	watchContext, cancelFunc := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelFunc()

	label, _ := kw.BuildLabel("builder", []string{site.ID.String()}) // TODO:
	podWatch, err := kw.GetImageBuilderWatcher(watchContext, label.String())
	if err != nil {
		return WatchResult{Err: err}
	}

	dataChan := make(chan WatchResult)

	go func(dataChan chan WatchResult) {
		for event := range podWatch.ResultChan() {
			p, ok := event.Object.(*corev1.Pod)
			if !ok {
				fmt.Println("unexpected type")
				continue
			}
			// Check Pod Phase. If its failed or succeeded.
			switch p.Status.Phase {
			case corev1.PodSucceeded:
				// TODO: Commit status to DB
				dataChan <- WatchResult{Status: string(constants.BuildSuccess), Reason: p.Status.Message, Err: nil}
				podWatch.Stop()
				break
			case corev1.PodFailed:
				// TODO: Commit status to DB with message
				fmt.Println("Image build failed. Reason : ", p.Status.Message)
				dataChan <- WatchResult{Status: string(constants.BuildFailed), Reason: p.Status.Message, Err: nil}
				podWatch.Stop()
				break
			}
		}
	}(dataChan)

	select {
	case <-watchContext.Done():
		return WatchResult{Status: string(constants.BuildFailed), Reason: "Watch Timeout", Err: nil}
	case x := <-dataChan:
		return x
	}
}

// Deletes the site's deployment and clusterIP service
func (fs *SiteService) DeleteSiteResources(
	kw *kuberneteswrapper.KubernetesWrapper,
	ctx context.Context,
	namespace string,
	deploymentName string,
	serviceName string,
) error {

	deploymentDeleteOptions := kuberneteswrapper.DeleteOptions{
		Ctx:       ctx,
		Name:      deploymentName,
		Namespace: namespace,
	}

	err := kw.DeleteDeployment(&deploymentDeleteOptions)
	if err != nil {
		return err
	}

	serviceDeleteOptions := kuberneteswrapper.DeleteOptions{
		Ctx:       ctx,
		Name:      serviceName,
		Namespace: namespace,
	}

	err = kw.DeleteService(&serviceDeleteOptions)
	if err != nil {
		return err
	}
	return nil
}

func (fs *SiteService) GetDeploymentLogs(
	kw *kuberneteswrapper.KubernetesWrapper,
	ctx context.Context,
	namespace string,
	deploymentName string,
	follow bool,
	rw http.ResponseWriter,
) error {

	label, _ := kw.BuildLabel("app", []string{deploymentName}) // TODO:

	pods, err := kw.KClient.CoreV1().
		Pods(namespace).
		List(ctx, metav1.ListOptions{LabelSelector: label.String()})

	var requests []struct {
		Request *rest.Request
		PodName string
	}
	for _, pod := range pods.Items {
		podlog := kw.KClient.CoreV1().
			Pods(namespace).
			GetLogs(pod.Name, &v1.PodLogOptions{Follow: true})
		requests = append(requests, struct {
			Request *rest.Request
			PodName string
		}{Request: podlog, PodName: pod.Name})
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(requests))
	for _, request := range requests {
		go func(req *rest.Request, podName string) {
			defer wg.Done()
			stream, err := req.Stream(ctx)
			if err != nil {
				return
			}
			defer stream.Close()
			for {
				buf := make([]byte, 2000)
				numBytes, err := stream.Read(buf)
				if err == io.EOF {
					break
				}
				if err != nil {
					return
				}
				if numBytes == 0 {
					time.Sleep(1 * time.Second)
					continue
				}
				message := string(buf[:numBytes])
				fmt.Fprintf(rw, "data: %v %v\n\n", podName, message)
				if f, ok := rw.(http.Flusher); ok {
					f.Flush()
				}
			}
			return
		}(request.Request, request.PodName)
	}
	wg.Wait()
	return err
}

func (fs *SiteService) DeleteImageBuilder(
	kw *kuberneteswrapper.KubernetesWrapper,
	ctx context.Context, namespace string,
) error {
	return kw.KClient.CoreV1().Pods(namespace).Delete(ctx, "kaniko-worker", metav1.DeleteOptions{})
}
