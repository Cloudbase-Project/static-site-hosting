package handlers

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
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

	urlString := r.URL.String()
	x := strings.Split(urlString, "/serve/"+siteId)

	siteURL := "http://cloudbase-ssh-" + siteId + "-svc:4000/static-site-hosting/serve/" + siteId + x[1]

	finalURL, err := url.Parse(siteURL)
	if err != nil {
		http.Error(rw, err.Error(), 400)
	}
	resp, err := http.Get(finalURL.String())
	fmt.Printf("resp: %v\n", resp.Body)
	defer resp.Body.Close()
	responseData, err := ioutil.ReadAll(resp.Body)

	rw.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	rw.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
	rw.Write(responseData)

}
