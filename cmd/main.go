package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ZhengjunHUO/k8s-sidecar-injection/pkg/server"
)

func main() {
	s := server.ServerInit()

	go s.Start()

	chInt := make(chan os.Signal, 1)
	signal.Notify(chInt, os.Interrupt, syscall.SIGTERM)

	<-chInt
	log.Println("Receive interrupt signal, stop server ...")
	s.Stop()
}
