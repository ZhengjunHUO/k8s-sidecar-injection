package server

import (
	"os"
	"os/signal"
	"syscall"
	"io"

	"net/http"
	"fmt"
	"log"
	"context"
)

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

func muteHandler(w http.ResponseWriter, r *http.Request) {
	// 1) read request's body into buffer
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		log.Fatalln(err)
	}
	// defer r.Body.Close()

	// 2) check body's content
	if len(buf) == 0 {
		log.Println("Empty body!")
		http.Error(w, "Empty body", http.StatusBadRequest)
		return
	}

	ct := r.Header.Get("Content-Type")
	if ct != "application/json" {
		log.Printf("Need application/json, receive: %s\n", ct)
		http.Error(w, "Expect application/json data", http.StatusBadRequest)
		return
	}

	fmt.Fprintf(w, "mutation handler to implement")
}
