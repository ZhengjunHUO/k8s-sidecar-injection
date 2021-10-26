package main

import (
	"github.com/ZhengjunHUO/k8s-sidecar-injection/pkg/server"
)

func main() {
	s := server.ServerInit()
	s.Run()
}
