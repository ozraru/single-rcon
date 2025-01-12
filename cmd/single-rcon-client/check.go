package main

import (
	"context"
	"log"
	"net"

	"golang.org/x/crypto/ssh"
)

func Check(ctx context.Context, conf *ConfigStruct) {
	log.Print("Checking validity...")

	signer, err := ssh.ParsePrivateKey([]byte(conf.Bridge.Privkey))
	if err != nil {
		log.Print("Failed to parse private key: ", err)
	}

	_, _, hostkey, _, _, err := ssh.ParseKnownHosts([]byte(conf.Bridge.Hostkey))
	if err != nil {
		log.Print("Failed to parse private key: ", err)
	}

	client, err := ssh.Dial("tcp", conf.Bridge.Address, &ssh.ClientConfig{
		User: conf.Bridge.Username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.FixedHostKey(hostkey),
	})
	if err != nil {
		log.Panic("Failed to connect server: ", err)
	}
	defer client.Close()

	if _, err := makeSSHServerConfig(ctx, conf); err != nil {
		log.Panic("Failed to make ssh config: ", err)
	}

	listener, err := client.ListenTCP(&net.TCPAddr{
		IP: net.IPv4zero,
	})
	if err != nil {
		log.Panic("Failed to listen: ", err)
	}
	defer listener.Close()

	log.Print("Check validity: SUCCESS")
}
