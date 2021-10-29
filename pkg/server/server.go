package server

import (
	"os"
	"os/signal"
	"syscall"
	"io"
	"net/http"
	"log"
	"context"

	admv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	INJECT_LABEL = "sidecar.huozj.io/inject"
	INJECT_STATUS = "sidecar.huozj.io/injected"
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
	go s.SetInterruptHandler(waitTillAllClosed)

	log.Println("[INFO] Starting server ...")
	if err := s.HttpServer.ListenAndServeTLS("server.crt", "server.key"); err != http.ErrServerClosed {
		log.Printf("[ERROR] Error bringing up the server: %v\n", err)
	}

	<-waitTillAllClosed
}

func (s *MuteServer) SetInterruptHandler(waitTillAllClosed chan struct{}) {
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

func updateReview(review *admv1beta1.AdmissionReview) {
	var pod corev1.Pod
	if err := json.Unmarshal(review.Request.Object.Raw, &pod); err != nil {
		log.Printf("[ERROR] Dump req's object failed: %s\n", err)
		return
	}

	// check if annotations match the injection criteria
	annos := pod.ObjectMeta.Annotations
	if annos == nil || annos[INJECT_STATUS] == "true" || annos[INJECT_LABEL] != "true" {
		log.Println("[INFO] Skip sidecar injection!")
		return
	}

	patch := []Patch_t{}

	CntPath, VolPath := "/spec/containers", "/spec/volumes"
	CntPathAppend, VolPathAppend := "/spec/containers/-", "/spec/volumes/-"
	hasContainer := len(pod.Spec.Containers) != 0
	hasVolume := len(pod.Spec.Volumes) != 0

	// patch sidecar container(s)
	for i := range Sidecarspec.Containers {
		if hasContainer {
			patch = append(patch, Patch_t{Op: "add", Path: CntPathAppend, Value: Sidecarspec.Containers[i],})
		}else{
			patch = append(patch, Patch_t{Op: "add", Path: CntPath, Value: []corev1.Container{Sidecarspec.Containers[i]},})
			hasContainer = false
		}
	}

	// patch sidecar volume(s)
	for i := range Sidecarspec.Volumes {
		if hasVolume {
			patch = append(patch, Patch_t{Op: "add", Path: VolPathAppend, Value: Sidecarspec.Volumes[i],})
		}else{
			patch = append(patch, Patch_t{Op: "add", Path: VolPath, Value: []corev1.Volume{Sidecarspec.Volumes[i]},})
			hasVolume = false
		}
	}

	// patch annotation
	if annos[INJECT_STATUS] == "" {
		patch = append(patch, Patch_t{Op: "add", Path: "/metadata/annotations/-", Value: map[string]string{ INJECT_STATUS: "true", },})
	}else{
		patch = append(patch, Patch_t{Op: "replace", Path: "/metadata/annotations/"+INJECT_STATUS, Value: "true",})
	}

	// serialize patch
	patchbuf, err := json.Marshal(patch)
	if err != nil {
		log.Printf("[ERROR] Serialization patch: %s\n", err)
		return
	}

	// write response
	var patchtype admv1beta1.PatchType = "JSONPatch"
	review.Response = &admv1beta1.AdmissionResponse{ UID: review.Request.UID, Allowed: true, Patch: patchbuf, PatchType: &patchtype, }

	log.Println("mutation handler to implement")
}

func muteHandler(w http.ResponseWriter, r *http.Request) {
	// 1) read request's body into buffer
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		log.Fatalln(err)
	}
	// defer r.Body.Close()

	// 2) check body's content
	if len(buf) == 0 {
		log.Println("[WARN] Empty body!")
		http.Error(w, "Empty body", http.StatusBadRequest)
		return
	}

	ct := r.Header.Get("Content-Type")
	if ct != "application/json" {
		log.Printf("[WARN] Need application/json type, received type: %s\n", ct)
		http.Error(w, "Expect application/json type data", http.StatusBadRequest)
		return
	}

	// 3) decode to AdmissionReview
	review := &admv1beta1.AdmissionReview{}
	decoder := serializer.NewCodecFactory(Scheme).UniversalDeserializer()
	_, _, err = decoder.Decode(buf, nil, review)
	if err != nil {
		log.Printf("[WARN] Decode body to admissionreview failed: %s\n", err)
		http.Error(w, "Json content malformed", http.StatusBadRequest)
		return
	}

	// 4) handle AdmissionReview
	updateReview(review)

	// 5) Prepare response
	rspbuf, err := json.Marshal(review)
	if err != nil {
		log.Printf("[ERROR] Serialize admissionreview failed: %s\n", err)
		http.Error(w, "Prepare response failed", http.StatusInternalServerError)
		return
	}

	if _, err := w.Write(rspbuf); err != nil {
		log.Printf("[ERROR] Serialize admissionreview failed: %s\n", err)
		http.Error(w, "Prepare response failed", http.StatusInternalServerError)
	}
}
