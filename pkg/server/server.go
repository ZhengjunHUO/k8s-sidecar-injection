package server

import (
	"net/http"
	"log"
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

func (s *MuteServer) Start() {
	if err := s.HttpServer.ListenAndServeTLS("/ssl/server.crt", "/ssl/server.key"); err != http.ErrServerClosed {
		log.Printf("Error bringing up the server: %v\n", err)
	}
}

func (s *MuteServer) Stop() {

}

func muteHandler(w http.ResponseWriter, r *http.Request) {

}
