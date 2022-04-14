package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/Cloudbase-Project/static-site-hosting/handlers"
	"github.com/Cloudbase-Project/static-site-hosting/middlewares"
	"github.com/Cloudbase-Project/static-site-hosting/models"
	"github.com/Cloudbase-Project/static-site-hosting/services"
)

func main() {

	logger := log.New(os.Stdout, "STATIC_SITE_HOSTING ", log.LstdFlags)

	err := godotenv.Load()
	if err != nil {
		logger.Fatal("Cannot load env variables")
	}

	PORT, ok := os.LookupEnv("PORT")
	if !ok {
		PORT = "3000"
	}

	router := mux.NewRouter()

	router.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte("hello world"))
	})

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	// dsn := "host=localhost user=gorm password=gorm dbname=gorm port=9920 sslmode=disable TimeZone=Asia/Shanghai"
	dsn := os.Getenv("POSTGRES_URI")
	fmt.Printf("dsn: %v\n", dsn)
	var db *gorm.DB

	for i := 0; i < 5; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			log.Println("failed to connect database")
			time.Sleep(time.Second * 10)
			continue
		}
		logger.Print("Connected to DB")
		break

	}

	db.AutoMigrate(&models.Site{}, &models.Config{})

	ss := services.NewSiteService(db, logger)
	cs := services.NewConfigService(db, logger)
	ps := services.NewProxyService(db, logger)

	site := handlers.NewSiteHandler(clientset, logger, ss)
	configHandler := handlers.NewConfigHandler(logger, cs)
	proxyHandler := handlers.NewProxyHandler(logger, ps)
	// add site
	router.HandleFunc("/site/{projectId}", middlewares.AuthMiddleware(site.CreateSite)).
		Methods(http.MethodPost)

	// list sites created by the user
	router.HandleFunc("/sites/{projectId}", middlewares.AuthMiddleware(site.ListSites)).
		Methods(http.MethodGet)

	// update site
	router.HandleFunc("/site/{projectId}/{siteId}/", middlewares.AuthMiddleware(site.UpdateSite)).
		Methods(http.MethodPatch)

	// delete site
	router.HandleFunc("/site/{projectId}/{siteId}", middlewares.AuthMiddleware(site.DeleteSite)).
		Methods(http.MethodDelete)

	// View a site. View status/replicas RPS etc
	router.HandleFunc("/site/{projectId}/{siteId}", middlewares.AuthMiddleware(site.GetSite)).
		Methods(http.MethodGet)

	// Get logs of a site
	router.HandleFunc("/site/{projectId}/{siteId}/logs", middlewares.AuthMiddleware(site.GetSiteLogs)).
		Methods(http.MethodGet)

	// Create site creates site image. User has to deploy/redeploy for deployments to take effect.
	router.HandleFunc("/site/{projectId}/{siteId}/deploy", middlewares.AuthMiddleware(site.DeploySite)).
		Methods(http.MethodPost)

	router.HandleFunc("/site/{projectId}/{siteId}/redeploy", middlewares.AuthMiddleware(site.RedeploySite)).
		Methods(http.MethodPost)

		// ------------------ CONFIG ROUTES
	router.HandleFunc("/config/", configHandler.CreateConfig).Methods(http.MethodPost)

	router.HandleFunc("/serve/{siteId}", proxyHandler.ProxyRequest).Methods(http.MethodGet)

	server := http.Server{
		Addr:    ":" + PORT,
		Handler: router,
	}

	// handle os signals to shutoff server
	c := make(chan os.Signal, 1)

	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Println("Starting server on port : ", PORT)
		logger.Fatal(server.ListenAndServe())
	}()

	<-c
	logger.Println("received signal. terminating...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	server.Shutdown(ctx)

}
