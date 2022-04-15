package utils

import (
	"encoding/json"
	"io"
	"net/http"
)

// returns a fully qualified image name given a site id.
//
// eg: quay.io/ubuntu-project/ubuntu:latest
func BuildImageName(siteId string) string {
	// TODO: implement this
	Registry := "ghcr.io"
	Project := "cloudbase-project"

	imageName := Registry + "/" + Project + "/" + siteId + ":latest"
	return imageName
}

func FromJSON(body io.Reader, value interface{}) interface{} {
	d := json.NewDecoder(body)
	return d.Decode(value)
}

// returns a service name given a siteId
//
// eg: 127319ey71e291y2e12e01u-srv
func BuildServiceName(siteId string) string {
	return "cloudbase-ssh-" + siteId + "-svc"
}

// set http headers
func SetSSEHeaders(rw http.ResponseWriter) http.ResponseWriter {
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	rw.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Connection", "keep-alive")
	return rw
}
