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
	hostname := ParseHostname(r.Host)
	target := h.Rules[hostname] + r.RequestURI

	log.Printf("%s %s%s -> %s\n", r.Method, hostname, r.RequestURI, target)

	req, err := http.NewRequest(r.Method, target, r.Body)
	if err != nil {
		log.Println(err)
		return
	}

	CopyHeaders(&r.Header, &req.Header)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println(err)
		return
	}

	header := w.Header()
	CopyHeaders(&res.Header, &header)
	w.WriteHeader(res.StatusCode)

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Println(err)
		return
	}

	if _, err := w.Write(body); err != nil {
		log.Println(err)
	}
}
