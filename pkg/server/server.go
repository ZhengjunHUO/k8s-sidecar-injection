package server

import (
	"os"
	"os/signal"
	"syscall"
	"io"
	"net/http"
	"log"
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/ZhengjunHUO/k8s-sidecar-injection/pkg/config"
)

const (
	INJECT_LABEL = "sidecar.huozj.io/inject"
	INJECT_STATUS = "sidecar.huozj.io/injected"
	INJECT_STATUS_PATH = "sidecar.huozj.io~1injected"
	CNTPATH = "/spec/containers"
	VOLPATH = "/spec/volumes"
	CNTPATHAPPEND = "/spec/containers/-"
	VOLPATHAPPEND = "/spec/volumes/-"
)

// http server to execute mutating operation
type MuteServer struct {
	HttpServer	*http.Server
}

// JSON patch struct
type Patch_t struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// sidecar to inject
type Sidecar_t struct {
	Containers []corev1.Container `yaml:"containers"`
	Volumes    []corev1.Volume    `yaml:"volumes"`
}

var Sidecarspec Sidecar_t

func init() {
	// Get sidecar's configuration
	fd, err := os.Open(config.Cfg.SidecarSpec)
	if err != nil {
		log.Fatalf("[ERROR] Open sidecar spec yaml: %s\n", err)
	}

	buf, err := io.ReadAll(fd)
	if err != nil {
		log.Fatalf("[ERROR] Read sidecar spec yaml: %s\n", err)
	}

	if err = yaml.Unmarshal(buf, &Sidecarspec); err != nil {
		log.Fatalf("[ERROR] Deserialize sidecar spec yaml: %s\n", err)
	}
}

func ServerInit() *MuteServer {
	// register handler to a http server
	smux := http.NewServeMux()
	smux.HandleFunc("/mutate", muteHandler)

	return &MuteServer {
		HttpServer: &http.Server{
			Addr:      ":443",
			Handler:   smux,
		},
	}
}

func (s *MuteServer) Run() {
	// trap interrupt signal in a goroutine
	waitTillAllClosed := make(chan struct{})
	go s.setInterruptHandler(waitTillAllClosed)

	// run the server with TLS, the certificate need to be issued by the k8s' CA
	log.Println("[INFO] Starting server ...")
	if err := s.HttpServer.ListenAndServeTLS(config.Cfg.ServerCert, config.Cfg.ServerKey); err != http.ErrServerClosed {
		log.Printf("[ERROR] Error bringing up the server: %v\n", err)
	}

	// shut down the server gracefully
	<-waitTillAllClosed
}

func (s *MuteServer) setInterruptHandler(waitTillAllClosed chan struct{}) {
	chInt := make(chan os.Signal, 1)
	signal.Notify(chInt, os.Interrupt, syscall.SIGTERM)

	<-chInt
	log.Println("[INFO] Receive interrupt signal, stop server ...")

	if err := s.HttpServer.Shutdown(context.Background()); err != nil {
		log.Printf("[ERROR] Error shutting server down: %v", err)
	}

	log.Println("[INFO] Server has been gracefully shut down !")
	close(waitTillAllClosed)
}
