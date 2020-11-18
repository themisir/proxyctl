package main

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"strings"
)

type Service struct {
	Name      string
	Target    string
	Namespace string
	Protocol  string
	Host      string
	Port      int
	LocalPort int
}

type Manifest struct {
	Listen   string
	Insecure bool
	Services []Service
	TLS      struct {
		Enabled bool
		Cert    string
		Key     string
	}
}

func ParseManifest(data []byte) (*Manifest, error) {
	manifest := Manifest{}
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	for i := range manifest.Services {
		svc := &manifest.Services[i]

		// Append ".local" to name if no gTLD is specified
		if !strings.Contains(svc.Name, ".") {
			svc.Name += ".local"
		}

		// Set namespace to "default" if not specified
		if svc.Namespace == "" {
			svc.Namespace = "default"
		}

		// Set local port to next available port if not specified
		if svc.LocalPort == 0 {
			svc.LocalPort = 1500 + i
		}

		// Set protocol to http if not specified
		if svc.Protocol == "" {
			svc.Protocol = "http"
		}

		// Set host to loopback if not specified
		if svc.Host == "" {
			svc.Host = "127.0.0.1"
		}
	}
	return &manifest, nil
}

func ReadManifest(filename string) (*Manifest, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return ParseManifest(data)
}