package handlers

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/Cloudbase-Project/static-site-hosting/services"
	"github.com/gorilla/mux"
)

type ProxyHandler struct {
	l       *log.Logger
	service *services.ProxyService
}

func NewProxyHandler(
	l *log.Logger,
	s *services.ProxyService,
) *ProxyHandler {
	return &ProxyHandler{l: l, service: s}
}

func (p *ProxyHandler) ProxyRequest(rw http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)

	siteId := vars["siteId"]

	_, err := p.service.VerifySite(siteId)
	if err != nil {
		http.Error(rw, err.Error(), 400)
	}

	urlString := r.URL.String()
	fmt.Printf("urlString: %v\n", urlString)
	x := strings.Split(urlString, "/serve/"+siteId)
	fmt.Println("xxxx : ", x)

	siteURL := "http://cloudbase-static-site-hosting-" + siteId + "-srv:4000" + x[0]
	fmt.Printf("siteURL: %v\n", siteURL)

	finalURL, err := url.Parse(siteURL)
	fmt.Printf("finalURL: %v\n", finalURL)
	if err != nil {
		http.Error(rw, err.Error(), 400)
	}
	fmt.Println("this")
	resp, err := http.Get(siteURL)
	fmt.Printf("resp: %v\n", resp)

	proxy := httputil.NewSingleHostReverseProxy(finalURL)
	r.URL.Host = finalURL.Host
	r.URL.Scheme = finalURL.Scheme
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Host = finalURL.Host
	r.URL.Path = finalURL.Path
	r.URL.RawPath = finalURL.RawPath
	proxy.ServeHTTP(rw, r)
	fmt.Println("after")

	// http://backend.cloudbase.dev/deploy/asdadjpiqwjdpqidjp/qwwe?123=qwe -> proxy to -> http://cloudbase-static-site-hosting-asdadjpiqwjdpqidjp-srv:4000qwwe?123=qwe

}
