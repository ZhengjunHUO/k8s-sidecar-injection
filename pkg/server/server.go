package server

import (
	"os"
	"os/signal"
	"syscall"

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
	fmt.Fprintf(w, "mutation handler to implement")
}
