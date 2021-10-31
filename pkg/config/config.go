package config

import (
	"flag"
)

type Config struct {
	SidecarSpec	string
	ServerCert	string
	ServerKey	string
}

var Cfg *Config

func init() {
	Cfg = &Config{}

	flag.StringVar(&(Cfg.SidecarSpec), "spec", "./sidecarspec.yaml", "Path to sidecar specification")
	flag.StringVar(&(Cfg.ServerCert), "cert", "./server.crt", "Path to server's certificate")
	flag.StringVar(&(Cfg.ServerKey), "key", "./server.key", "Path to server's private key")
}
