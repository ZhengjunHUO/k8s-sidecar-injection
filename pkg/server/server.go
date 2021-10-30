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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	INJECT_LABEL = "sidecar.huozj.io/inject"
	INJECT_STATUS = "sidecar.huozj.io/injected"
	CNTPATH = "/spec/containers"
	VOLPATH = "/spec/volumes"
	CNTPATHAPPEND = "/spec/containers/-"
	VOLPATHAPPEND = "/spec/volumes/-"
)

type MuteServer struct {
	HttpServer	*http.Server
}

type Patch_t struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

type Sidecar_t struct {
	Containers []corev1.Container `yaml:"containers"`
	Volumes    []corev1.Volume    `yaml:"volumes"`
}

var Sidecarspec Sidecar_t

var Scheme = runtime.NewScheme()

func init() {
	fd, err := os.Open("sidecarspec.yaml")
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
	waitTillAllClosed := make(chan struct{})
	go s.setInterruptHandler(waitTillAllClosed)

	log.Println("[INFO] Starting server ...")
	if err := s.HttpServer.ListenAndServeTLS("server.crt", "server.key"); err != http.ErrServerClosed {
		log.Printf("[ERROR] Error bringing up the server: %v\n", err)
	}

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
