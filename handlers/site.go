package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"

	kuberneteswrapper "github.com/Cloudbase-Project/static-site-hosting/KubernetesWrapper"
	"github.com/Cloudbase-Project/static-site-hosting/constants"
	"github.com/Cloudbase-Project/static-site-hosting/dtos"
	"github.com/Cloudbase-Project/static-site-hosting/models"
	"github.com/Cloudbase-Project/static-site-hosting/services"
	"github.com/Cloudbase-Project/static-site-hosting/utils"
	"github.com/gorilla/mux"

	"k8s.io/client-go/kubernetes"
)

type SiteHandler struct {
	l       *log.Logger
	service *services.SiteService
	kw      *kuberneteswrapper.KubernetesWrapper
	mu      *sync.Mutex
	fq      *[]string
}

// create new site
func NewSiteHandler(
	client *kubernetes.Clientset,
	l *log.Logger,
	s *services.SiteService,
	mu *sync.Mutex,
	fq *[]string,
) *SiteHandler {
	kw := kuberneteswrapper.NewWrapper(client)
	return &SiteHandler{l: l, service: s, kw: kw, mu: mu, fq: fq}
}

// Get all sites created by this user.
func (f *SiteHandler) ListSites(rw http.ResponseWriter, r *http.Request) {

	ownerId := r.Context().Value("ownerId").(string)
	vars := mux.Vars(r)

	projectId := vars["projectId"]

	sites, err := f.service.GetAllSites(ownerId, projectId)
	if err != nil {
		http.Error(rw, err.Error(), 500)
	}

	err = sites.ToJSON(rw)
	if err != nil {
		http.Error(rw, "Unable to marshal JSON", http.StatusInternalServerError)
	}
}

// Get a site given a "siteId" in the route params
func (f *SiteHandler) GetSite(rw http.ResponseWriter, r *http.Request) {
	ownerId := r.Context().Value("ownerId").(string)

	vars := mux.Vars(r)
	projectId := vars["projectId"]

	site, err := f.service.GetSite(vars["siteId"], ownerId, projectId)
	if err != nil {
		http.Error(rw, err.Error(), 500)
	}

	err = site.ToJSON(rw)
	if err != nil {
		http.Error(rw, "Unable to marshal JSON", http.StatusInternalServerError)
	}
}

func (f *SiteHandler) UpdateSite(rw http.ResponseWriter, r *http.Request) {
	// set status to readyToDeploy
	// set LastAction to update

	ownerId := r.Context().Value("ownerId").(string)

	vars := mux.Vars(r)
	projectId := vars["projectId"]

	var data *dtos.UpdateCodeDTO
	utils.FromJSON(r.Body, data)

	if _, err := dtos.Validate(data); err != nil {
		http.Error(rw, "Validation error", 400)
		return
	}

	// get the site.
	site, err := f.service.GetSite(vars["siteId"], ownerId, projectId)
	if err != nil {
		http.Error(rw, err.Error(), 500)

	}

	// site.Code = data.Code
	site.BuildStatus = string(constants.Building)
	// save it
	f.service.SaveSite(site)

	imageName := utils.BuildImageName(site.ID.String())

	// build image
	f.kw.CreateImageBuilder(&kuberneteswrapper.ImageBuilder{
		Ctx:       r.Context(),
		Namespace: constants.Namespace,
		SiteId:    site.ID.String(),
		ImageName: imageName,
	})

	rw.Write([]byte("Building new image for your updated code"))

	result := f.service.WatchImageBuilder(f.kw, site, constants.Namespace)
	if result.Err != nil {
		// http.Error(rw, "Error watching image builder", 500)
		f.l.Print("error watching image builder", result.Err)
	}

	site.BuildFailReason = result.Reason
	site.BuildStatus = result.Status
	// TODO: Should Come back to this. maybe have to add lastAction  = update
	site.LastAction = string(constants.UpdateAction)
	site.DeployStatus = string(constants.RedeployRequired)
	f.service.SaveSite(site)

}

func (f *SiteHandler) DeleteSite(rw http.ResponseWriter, r *http.Request) {
	// get site from db
	vars := mux.Vars(r)

	siteId := vars["siteId"]
	ownerId := r.Context().Value("ownerId").(string)

	projectId := vars["projectId"]

	// delete it.
	err := f.service.DeleteSite(siteId, ownerId, projectId)
	if err != nil {
		f.l.Print(err)
		http.Error(rw, "DB error", 500)
	}
	// TODO: delete resources
	serviceName := utils.BuildServiceName(siteId)

	err = f.service.DeleteSiteResources(
		f.kw,
		context.Background(),
		constants.Namespace,
		siteId,
		serviceName,
	)
	if err != nil {
		f.l.Print(err)
		http.Error(rw, "Err deleting resources", 500)
	}
	// TODO: Remove from router
}

func (f *SiteHandler) GetSiteLogs(rw http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	ownerId := r.Context().Value("ownerId").(string)

	projectId := vars["projectId"]

	// get site
	site, err := f.service.GetSite(vars["siteId"], ownerId, projectId)
	if err != nil {
		http.Error(rw, "Error getting site, "+err.Error(), 400)
	}

	if site.DeployStatus == string(constants.Deployed) &&
		site.LastAction == string(constants.DeployAction) {
		// get the logs for the given site
		err := f.service.GetDeploymentLogs(
			f.kw,
			r.Context(),
			constants.Namespace,
			site.ID.String(),
			true,
			rw,
		)
		if err != nil {
			http.Error(rw, "Error getting logs"+err.Error(), 500)
		}

		if f, ok := rw.(http.Flusher); ok {
			f.Flush()
		}

	} else {
		http.Error(rw, "Cannot perform this action currently", 400)
	}
	// check if its deployStatus is deployed
	// check if lastAction is deploy
	// check if
}

/*
Create a deployment and a clusterIP service for the site.

Errors if no image is found for the site
*/
func (f *SiteHandler) DeploySite(rw http.ResponseWriter, r *http.Request) {

	//  Get site from db.
	vars := mux.Vars(r)
	ownerId := r.Context().Value("ownerId").(string)

	projectId := vars["projectId"]

	site, err := f.service.GetSite(vars["siteId"], ownerId, projectId)
	if err != nil {
		http.Error(rw, "DB error", 500)
	}

	if site.BuildStatus == string(constants.BuildSuccess) &&
		site.DeployStatus == string(constants.NotDeployed) &&
		site.LastAction == string(constants.BuildAction) {
		// proceed

		deploymentLabel := map[string]string{"app": site.ID.String()}

		// TODO: Should change to constant
		replicas := int32(1)

		imageName := utils.BuildImageName(site.ID.String())

		err = f.service.DeploySite(
			f.kw,
			r.Context(),
			constants.Namespace,
			site.ID.String(),
			deploymentLabel,
			imageName,
			replicas,
		)
		if err != nil {
			fmt.Printf("err: %v\n", err.Error())
			http.Error(rw, "Error deploying your image.", 500)
			return
		}

		// update status in db
		site.DeployStatus = string(constants.Deploying)
		f.service.SaveSite(site)

		// rw.Write([]byte("Deploying your site..."))
		rw = utils.SetSSEHeaders(rw)
		fmt.Fprintf(rw, "data: %v\n\n", "Deploying your site...")

		if f, ok := rw.(http.Flusher); ok {
			f.Flush()
		}

		// Watch status
		// watch for 1 min and then close everything

		result := f.service.WatchDeployment(f.kw, site, constants.Namespace)
		if result.Err != nil {
			http.Error(rw, "Error watching deployment", 500)
		}

		site.DeployFailReason = result.Reason
		site.DeployStatus = result.Status
		site.LastAction = string(constants.DeployAction)
		f.service.SaveSite(site)

		// TODO: register with the custom router
		fmt.Fprintf(rw, "data: %v\n\n", "Deployed your site successfully")

	} else {
		http.Error(rw, "Cannot perform this action currently", 400)
	}

}

func (f *SiteHandler) GetFromQueue(rw http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	// *f.fq = append(*f.fq, site.ID.String())
	var fileString string
	fileString, *f.fq = (*f.fq)[0], (*f.fq)[1:]
	f.mu.Unlock()
	http.ServeFile(rw, r, "./zipfiles/"+fileString+".zip")
}

func (f *SiteHandler) CreateSite(rw http.ResponseWriter, r *http.Request) {

	// TODO: 1. authenicate and get userId
	// TODO: 2. check if the service is enabled

	// FILE UPLOAD HANDLING
	r.ParseMultipartForm(10 << 20)
	file, handler, err := r.FormFile("myFile")
	if err != nil {
		fmt.Println("Error Retrieving the File")
		fmt.Println(err)
		return
	}
	defer file.Close()
	fmt.Printf("Uploaded File: %+v\n", handler.Filename)
	fmt.Printf("File Size: %+v\n", handler.Size)
	fmt.Printf("MIME Header: %+v\n", handler.Header)

	ownerId := r.Context().Value("ownerId").(string)

	vars := mux.Vars(r)
	projectId := vars["projectId"]

	// Commit to db
	// TODO:
	site, err := f.service.CreateSite(ownerId, projectId)
	if err != nil {
		http.Error(rw, "DB error", 500)
	}
	fmt.Printf("site: %v\n", site)

	tempFile, err := ioutil.TempFile("zipfiles", site.ID.String()+".zip")
	if err != nil {
		fmt.Println(err)
	}
	defer tempFile.Close()

	// read all of the contents of our uploaded file into a
	// byte array
	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println(err)
	}
	// write this byte array to our temporary file
	tempFile.Write(fileBytes)
	// return that we have successfully uploaded our file!
	fmt.Fprintf(rw, "Successfully Uploaded File\n")

	f.mu.Lock()
	*f.fq = append(*f.fq, site.ID.String())
	f.mu.Unlock()
	// if err != nil {
	// 	http.Error(rw, "cannot read json", 400)
	// }

	// TODO: get these from env variables
	Registry := os.Getenv("REGISTRY")
	Project := os.Getenv("PROJECT_NAME")

	imageName := Registry + "/" + Project + "/" + site.ID.String() + ":latest"
	// imageName := Registry + "/" + Project + "/" + "test1" + ":latest"
	fmt.Printf("imageName: %v\n", imageName)
	// _, err := f.kw.CreateNamespace(r.Context(), constants.Namespace)

	// create namespace if not exist
	if err != nil {
		// namespace already exists. ignore
		fmt.Println("namespace already exists. ignoring...")
		fmt.Printf("err: %v\n", err)
	}

	// create kaniko pod

	_, err = f.kw.CreateImageBuilder(
		&kuberneteswrapper.ImageBuilder{
			Ctx:       r.Context(),
			Namespace: constants.Namespace,
			SiteId:    site.ID.String(),
			ImageName: imageName,
		})

	if err != nil {
		http.Error(rw, "erroror : "+err.Error(), 400)
	}

	// podLogs = clientset.CoreV1().Pods("static-site-hosting").GetLogs("kaniko-worker", &v1.PodLogOptions{})

	rw = utils.SetSSEHeaders(rw)

	// rw.Write([]byte("Building Image for your code"))
	fmt.Fprintf(rw, "data: %v\n\n", "Building Image for your code")

	if f, ok := rw.(http.Flusher); ok {
		f.Flush()
	}

	result := f.service.WatchImageBuilder(f.kw, site, constants.Namespace)
	if result.Err != nil {
		http.Error(rw, "Error watching image builder", 500)
		fmt.Println("err : ", result.Err.Error())
	}

	err = f.service.DeleteImageBuilder(f.kw, r.Context(), constants.Namespace)
	if err != nil {
		fmt.Printf("err deleting image builder: %v\n", err.Error())
	}
	site.BuildFailReason = result.Reason
	site.BuildStatus = result.Status
	site.LastAction = string(constants.BuildAction)
	f.service.SaveSite(site)
	resp := struct {
		Site    models.Site
		Message string
	}{
		Site:    *site,
		Message: "Built image for Site",
	}

	json.NewEncoder(rw).Encode(resp)
}

func (f *SiteHandler) RedeploySite(rw http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ownerId := r.Context().Value("ownerId").(string)

	projectId := vars["projectId"]

	site, err := f.service.GetSite(vars["siteId"], ownerId, projectId)
	if err != nil {
		http.Error(rw, "DB error", 500)
	}

	if site.LastAction == string(constants.UpdateAction) &&
		site.DeployStatus == string(constants.RedeployRequired) &&
		site.BuildStatus == string(constants.BuildSuccess) {
		// proceed

		err = f.kw.UpdateDeployment(&kuberneteswrapper.UpdateOptions{
			Ctx:       context.Background(),
			Namespace: constants.Namespace,
			Name:      site.ID.String(),
		})
		if err != nil {
			f.l.Print(err)
			http.Error(rw, "error occured when redeploying", 500)
		}
		rw.Write([]byte("Deploying your code..."))

	} else {
		http.Error(rw, "Cannot perform this action.", 400)
	}
}
