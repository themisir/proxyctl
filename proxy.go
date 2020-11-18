package main

import (
	"crypto/tls"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

type ProxyHandler struct {
	Rules  map[string]string
}

func DisableSSL() {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
}

func ParseHostname(s string) string {
	parts := strings.Split(s, ":")
	return parts[0]
}

func CopyHeaders(a *http.Header, b *http.Header) {
	for k, v := range *a {
		for _, s := range v {
			b.Add(k, s)
		}
	}
}

func (h ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Prepare
	hostname := ParseHostname(r.Host)
	target := h.Rules[hostname] + r.RequestURI
	checkError := func(err error) bool {
		if err != nil {
			log.Println(err)
			w.WriteHeader(503)
			_, _ = w.Write(nil)
		}
		return err != nil
	}

	// Log request
	log.Printf("%s %s%s -> %s\n", r.Method, hostname, r.RequestURI, target)

	// Prepare request
	req, err := http.NewRequest(r.Method, target, r.Body); if checkError(err) {
		return
	}
	CopyHeaders(&r.Header, &req.Header)

	// Execute request
	res, err := http.DefaultClient.Do(req); if checkError(err) {
		return
	}

	// Reply headers
	header := w.Header()
	CopyHeaders(&res.Header, &header)
	w.WriteHeader(res.StatusCode)

	// Reply body
	body, err := ioutil.ReadAll(res.Body); if checkError(err) {
		return
	}
	if _, err = w.Write(body); err != nil {
		log.Println(err)
	}
}
