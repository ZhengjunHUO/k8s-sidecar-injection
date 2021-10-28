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
)

var Scheme = runtime.NewScheme()

type MuteServer struct {
	HttpServer	*http.Server
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
	if annos == nil || annos["sidecar.huozj.io/injected"] == "true" || annos["sidecar.huozj.io/inject"] != "true" {
		log.Println("[INFO] Skip sidecar injection!")
		return
	}

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
