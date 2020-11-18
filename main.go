package main

import (
	"flag"
	"fmt"
	"github.com/lextoumbourou/goodhosts"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var childProcesses = make([]*os.Process, 0)
var verbose = false

func StartKubectlProxy(svc *Service) error {
	if verbose {
		log.Printf("Starting %s (%s) proxy at *:%d", svc.Target, svc.Namespace, svc.LocalPort)
	}

	cmd := exec.Command(
		"kubectl",
		"port-forward",
		"--namespace",
		svc.Namespace,
		svc.Target,
		fmt.Sprintf("%d:%d", svc.LocalPort, svc.Port),
	)

	err := cmd.Start()

	if err == nil {
		childProcesses = append(childProcesses, cmd.Process)
	}

	return err
}

func GetManifestFilename() string {
	if flag.NArg() > 0 {
		return flag.Arg(0)
	}

	tryFiles := []string{"proxy.yaml", "proxy.yml"}

	if usr, err := user.Current(); err == nil {
		tryFiles = append(
			tryFiles,
			path.Join(usr.HomeDir, "proxy.yaml"),
			path.Join(usr.HomeDir, ".proxy.yaml"),
			path.Join(usr.HomeDir, "proxy.yml"),
			path.Join(usr.HomeDir, ".proxy.yml"),
		)
	}

	for _, filename := range tryFiles {
		if _, err := os.Stat(filename); err == nil {
			return filename
		}
	}

	log.Println("Manifest file is not defined.")
	Exit(1)
	return ""
}

func Exit(code int) {
	for _, proc := range childProcesses {
		if err := proc.Kill(); err != nil {
			log.Println(err)
		}
	}
	os.Exit(code)
}

func ListenSignals() {
	channel := make(chan os.Signal, 1)
	signal.Notify(channel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGQUIT)
	go func() {
		s := <-channel
		log.Println("Got signal", s)
		Exit(1)
	}()
}

func main() {
	ListenSignals()

	noHosts := false
	listen := ""
	insecure := false

	flag.BoolVar(&noHosts, "no-hosts", false, "Disable hosts file changes")
	flag.StringVar(&listen, "listen", "", "Listen address")
	flag.BoolVar(&insecure, "insecure", false, "Disable SSL certificate verification")
	flag.BoolVar(&verbose, "verbose", false, "Print detailed logs")
	flag.Parse()

	manifest, err := ReadManifest(GetManifestFilename()); if err != nil {
		panic(err)
	}
	hosts, err := goodhosts.NewHosts(); if err != nil {
		panic(err)
	}
	rules := make(map[string]string, len(manifest.Services))
	hostsModified := false

	if listen == "" {
		listen = manifest.Listen
	}

	port, err := strconv.ParseInt(strings.Split(listen, ":")[1], 10, 32); if err != nil {
		log.Println("Invalid port number")
		Exit(1)
	}

	println("Proxyctl will redirect requests from those domains:")

	for _, svc := range manifest.Services {
		// Update proxy rule
		rules[svc.Name] = fmt.Sprintf("%s://%s:%d", svc.Protocol, svc.Host, svc.LocalPort)

		// Start kubectl port-forward proxy process
		if err := StartKubectlProxy(&svc); err != nil {
			log.Println(err)
			Exit(2)
		}

		// Add entry to hosts file
		if !noHosts && !hosts.Has("127.0.0.1", svc.Name) {
			hostsModified = true
			hosts.Lines = append(hosts.Lines, goodhosts.HostsLine{
				IP:    "127.0.0.1",
				Hosts: []string{svc.Name},
				Raw:   fmt.Sprintf("127.0.0.1 %s", svc.Name),
			})
		}

		// Print URL
		if manifest.TLS.Enabled {
			if port == 443 {
				fmt.Printf("https://%s/\n", svc.Name)
			} else {
				fmt.Printf("https://%s:%d/\n", svc.Name, port)
			}
		} else {
			if port == 80 {
				fmt.Printf("http://%s/\n", svc.Name)
			} else {
				fmt.Printf("http://%s:%d/\n", svc.Name, port)
			}
		}
	}

	if hostsModified {
		if err := hosts.Flush(); err != nil {
			log.Println("Failed to write hosts file.", err)
			Exit(2)
		}
	}

	// Skip SSL verification if --insecure flag passed
	if insecure || manifest.Insecure {
		DisableSSL()
	}

	// Wait a seconds for kubectl proxies to fire up
	time.Sleep(1 * time.Second)

	// Create handler
	handler := ProxyHandler{rules}

	// Listen and serve
	log.Printf("Listening at %s\n", listen)
	if manifest.TLS.Enabled {
		if err := http.ListenAndServeTLS(listen, manifest.TLS.Cert, manifest.TLS.Key, handler); err != nil {
			log.Println(err)
			Exit(2)
		}
	} else {
		if err := http.ListenAndServe(listen, handler); err != nil {
			log.Println(err)
			Exit(2)
		}
	}
}
