package main

import (
	"context"
	"log"
	"os"
	"os/signal"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	go func() {
		<-sigCh
		cancel()
	}()

	conf, err := loadConfig(ctx)
	if err != nil {
		log.Fatal("Failed to load config: ", err)
	}

	if len(os.Args) < 2 {
		Default(ctx, conf)
		return
	}
	switch os.Args[1] {
	case "check":
		Check(ctx, conf)
	case "run":
		Run(ctx, conf)
	case "install":
		Install(ctx, conf)
	case "uninstall":
		Uninstall(ctx, conf)
	}
}
