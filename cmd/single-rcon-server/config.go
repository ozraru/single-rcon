package main

import (
	"context"
	"os"

	"github.com/goccy/go-yaml"
)

type ConfigStruct struct {
	Listen  string
	Clients map[string]ClientConfig
}

type ClientConfig struct {
	Key    string
	Listen string
}

func loadConfig(ctx context.Context) (*ConfigStruct, error) {
	f, err := os.Open("server-config.yaml")
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(f)
	conf := ConfigStruct{}
	return &conf, dec.DecodeContext(ctx, &conf)
}
