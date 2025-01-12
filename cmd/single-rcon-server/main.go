package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"slices"

	"golang.org/x/crypto/ssh"
)

func main() {
	ctx := context.Background()
	conf, err := loadConfig(ctx)
	if err != nil {
		log.Panic("Failed to load config: ", err)
	}

	sshConf, err := makeSSHConfig(ctx, conf)
	if err != nil {
		log.Panic("Failed to make ssh config: ", err)
	}

	listener, err := net.Listen("tcp", conf.Listen)
	if err != nil {
		log.Panic("Failed to listen: ", err)
	}
	defer listener.Close()

	log.Print("Listening...")

	for {
		netConn, err := listener.Accept()
		if err != nil {
			log.Panic("Failed to accept: ", err)
		}
		defer netConn.Close()

		connectionHandler(ctx, conf, netConn, sshConf)
	}
}

func makeSSHConfig(ctx context.Context, conf *ConfigStruct) (*ssh.ServerConfig, error) {
	sshConf := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			client, ok := conf.Clients[conn.User()]
			if !ok {
				return nil, errors.New("Unknown user")
			}
			confKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(client.Key))
			if err != nil {
				return nil, err
			}
			if !slices.Equal(key.Marshal(), confKey.Marshal()) {
				return nil, errors.New("key does not match")
			}
			return nil, nil
		},
	}

	if keyfile, err := os.OpenFile("hostkey", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600); err == nil {
		_, hostkeypriv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			log.Panic("Failed to generate key: ", err)
		}

		keydata, err := ssh.MarshalPrivateKey(hostkeypriv, "single-rcon generated host key")

		if err := pem.Encode(keyfile, keydata); err != nil {
			return nil, err
		}
	}

	keyfile, err := os.Open("hostkey")
	if err != nil {
		return nil, err
	}

	keydata, err := io.ReadAll(keyfile)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(keydata)
	if err != nil {
		return nil, err
	}

	sshConf.AddHostKey(signer)

	return sshConf, nil
}

func connectionHandler(ctx context.Context, conf *ConfigStruct, netConn net.Conn, sshConf *ssh.ServerConfig) {

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	conn, channelCh, requestCh, err := ssh.NewServerConn(netConn, sshConf)
	if err != nil {
		log.Print("Failed to make new server conn: ", err)
		return
	}
	defer conn.Close()

	go func() {
		for {
			select {
			case channel, ok := <-channelCh:
				if !ok {
					cancel()
					return
				}
				channel.Reject(ssh.Prohibited, "this server not allow any channel")
			case request, ok := <-requestCh:
				if !ok {
					cancel()
					return
				}
				if !request.WantReply {
					continue
				}
				if request.Type != "tcpip-forward" {
					request.Reply(false, nil)
					continue
				}
				var channelForwardMsg struct {
					Addr  string
					Rport uint32
				}
				if err := ssh.Unmarshal(request.Payload, &channelForwardMsg); err != nil {
					log.Print("Failed to unmarshal payload: ", err)
					request.Reply(false, nil)
					continue
				}
				listenAddr, err := net.ResolveTCPAddr("tcp", conf.Clients[conn.User()].Listen)
				if channelForwardMsg.Rport != 0 && listenAddr.Port != int(channelForwardMsg.Rport) {
					log.Print("Requested forbidden port forward: ", channelForwardMsg.Rport)
					request.Reply(false, nil)
					continue
				}

				listener, err := net.ListenTCP("tcp", listenAddr)
				if err != nil {
					log.Print("Failed to listen port: ", err)
					request.Reply(false, nil)
					continue
				}
				defer listener.Close()
				go func() {
					for {
						tcpConn, err := listener.AcceptTCP()
						if err != nil {
							log.Print("Failed to accept tcp: ", err)
							return
						}
						go func() {
							defer tcpConn.Close()
							ctx, cancelConn := context.WithCancel(ctx)
							defer cancelConn()
							type forwardedTCPPayload struct {
								Addr       string
								Port       uint32
								OriginAddr string
								OriginPort uint32
							}
							localAddr := tcpConn.LocalAddr().(*net.TCPAddr)
							remoteAddr := tcpConn.RemoteAddr().(*net.TCPAddr)
							channel, reqCh, err := conn.OpenChannel("forwarded-tcpip", ssh.Marshal(forwardedTCPPayload{
								Addr:       channelForwardMsg.Addr,
								Port:       uint32(localAddr.Port),
								OriginAddr: remoteAddr.IP.String(),
								OriginPort: uint32(remoteAddr.Port),
							}))
							if err != nil {
								log.Print("Failed to open channel to forward tcpip: ", err)
								return
							}
							defer channel.Close()
							go func() {
								io.Copy(channel, tcpConn)
								cancelConn()
							}()
							go func() {
								io.Copy(tcpConn, channel)
								cancelConn()
							}()
							go func() {
								for req := range reqCh {
									if req.WantReply {
										req.Reply(false, nil)
									}
								}
								cancelConn()
							}()
							<-ctx.Done()
						}()
					}
				}()
				if channelForwardMsg.Rport == 0 {
					type p struct {
						Port uint32
					}
					request.Reply(true, ssh.Marshal(p{
						Port: uint32(listenAddr.Port),
					}))
				} else {
					request.Reply(true, nil)
				}
				continue
			case <-ctx.Done():
				return
			}
		}
	}()

	log.Print("Connection closed: ", conn.Wait())
}
