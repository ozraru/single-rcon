package main

import (
	"context"
	"os"

	"github.com/goccy/go-yaml"
)

type ConfigStruct struct {
	Install string
	Bridge  BridgeConfig
	Server  ServerConfig
}

type BridgeConfig struct {
	Address  string
	Hostkey  string
	Username string
	Privkey  string
}

type ServerConfig struct {
	Users map[string]UserConfig
}

type UserConfig struct {
	Key string
}

func loadConfig(ctx context.Context) (*ConfigStruct, error) {
	f, err := os.Open("client-config.yaml")
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(f)
	conf := ConfigStruct{}
	return &conf, dec.DecodeContext(ctx, &conf)
}
