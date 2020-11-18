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
	"syscall"
	"time"
)

var childProcesses = make([]*os.Process, 0)

func StartKubectlProxy(svc *Service) error {
	log.Printf("Starting %s (%s) proxy at *:%d", svc.Target, svc.Namespace, svc.LocalPort)

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

	if usr, err := user.Current(); err != nil && usr != nil {
		tryFiles = append(tryFiles, path.Join(usr.HomeDir, ".proxy.yaml"), path.Join(usr.HomeDir, ".proxy.yml"))
	}

	for _, filename := range tryFiles {
		if _, err := os.Stat(filename); err != nil {
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
	flag.Parse()

	manifest, err := ReadManifest(GetManifestFilename()); if err != nil {
		panic(err)
	}
	hosts, err := goodhosts.NewHosts(); if err != nil {
		panic(err)
	}
	rules := make(map[string]string, len(manifest.Services))
	hostsModified := false

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

	// Wait 3 seconds for kubectl proxies to fire up
	time.Sleep(3 * time.Second)

	if listen == "" {
		listen = manifest.Listen
	}

	log.Printf("Listening at %s\n", listen)

	if err := http.ListenAndServe(listen, ProxyHandler{rules}); err != nil {
		log.Println(err)
		Exit(2)
	}
}
