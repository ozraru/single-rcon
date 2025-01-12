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
	"os/exec"
	"slices"

	"github.com/creack/pty"
	"golang.org/x/crypto/ssh"
)

func Run(ctx context.Context, conf *ConfigStruct) {
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

	sshConf, err := makeSSHServerConfig(ctx, conf)
	if err != nil {
		log.Panic("Failed to make ssh config: ", err)
	}

	listener, err := client.ListenTCP(&net.TCPAddr{
		IP: net.IPv4zero,
	})
	if err != nil {
		log.Panic("Failed to listen: ", err)
	}
	defer listener.Close()

	for {
		netConn, err := listener.Accept()
		if err != nil {
			log.Panic("Failed to accept: ", err)
		}
		defer netConn.Close()

		connectionHandler(ctx, netConn, sshConf)
	}
}

func makeSSHServerConfig(ctx context.Context, conf *ConfigStruct) (*ssh.ServerConfig, error) {
	sshConf := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			user, ok := conf.Server.Users[conn.User()]
			if !ok {
				return nil, errors.New("Unknown user")
			}
			confKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(user.Key))
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

func connectionHandler(ctx context.Context, netConn net.Conn, sshConf *ssh.ServerConfig) {

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	conn, channelCh, globalRequestCh, err := ssh.NewServerConn(netConn, sshConf)
	if err != nil {
		log.Print("Failed to make new server conn: ", err)
		return
	}
	defer conn.Close()

	go func() {
		for {
			select {
			case newChannel, ok := <-channelCh:
				if !ok {
					cancel()
					return
				}
				if newChannel.ChannelType() != "session" {
					log.Print("Invalid channel type: ", newChannel.ChannelType())
					newChannel.Reject(ssh.Prohibited, "this server allow only session channel")
				}
				channel, requestCh, err := newChannel.Accept()
				if err != nil {
					log.Print("Failed to accept channel: ", err)
				}
				defer channel.Close()
				programStarted := false
				var ptyFile *os.File
				var ptySize *pty.Winsize
				go func() {
					for req := range requestCh {
						switch req.Type {
						case "shell":
							if programStarted {
								log.Print("Duplicate program run: ", err)
								if req.WantReply {
									req.Reply(false, nil)
								}
								continue
							}
							programStarted = true
							cmd := exec.Command("sh")
							if ptySize == nil {
								cmd.Stdin = channel
								cmd.Stdout = channel
								cmd.Stderr = channel.Stderr()
								err := cmd.Start()
								if err != nil {
									log.Print("Failed to start shell: ", err)
									if req.WantReply {
										req.Reply(false, nil)
									}
									continue
								}
							} else {
								ptyFile, err = pty.StartWithSize(cmd, ptySize)
								if err != nil {
									log.Print("Failed to start shell: ", err)
									if req.WantReply {
										req.Reply(false, nil)
									}
									continue
								}
								go io.Copy(channel, ptyFile)
								go io.Copy(ptyFile, channel)
								if req.WantReply {
									req.Reply(true, nil)
								}
							}
							go func() {
								err := cmd.Wait()
								log.Print("shell closed: ", err)
								type ExitStatus struct {
									Code uint32
								}
								if err == nil {
									channel.SendRequest("exit-status", false, ssh.Marshal(ExitStatus{
										Code: 0,
									}))
								} else if exitErr, ok := err.(*exec.ExitError); ok {
									channel.SendRequest("exit-status", false, ssh.Marshal(ExitStatus{
										Code: uint32(exitErr.ExitCode()),
									}))
								}
								channel.Close()
							}()
						case "exec":
							log.Print("unsupported exec")
						case "pty-req":
							var payload struct {
								TerminalEnv  string
								CharWidth    uint32
								CharHeight   uint32
								PixelWidth   uint32
								PixelHeight  uint32
								TerminalMode string
							}
							if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
								log.Print("Failed to parse pty-req: ", err)
								if req.WantReply {
									req.Reply(false, nil)
								}
							}
							ptySize = &pty.Winsize{
								Rows: uint16(payload.CharHeight),
								Cols: uint16(payload.CharWidth),
								X:    uint16(payload.PixelWidth),
								Y:    uint16(payload.PixelHeight),
							}
							if req.WantReply {
								req.Reply(true, nil)
							}
						case "window-change":
							var payload struct {
								CharWidth   uint32
								CharHeight  uint32
								PixelWidth  uint32
								PixelHeight uint32
							}
							if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
								log.Print("Failed to parse window-change: ", err)
							}
							ptySize = &pty.Winsize{
								Rows: uint16(payload.CharHeight),
								Cols: uint16(payload.CharWidth),
								X:    uint16(payload.PixelWidth),
								Y:    uint16(payload.PixelHeight),
							}
							if ptyFile != nil {
								pty.Setsize(ptyFile, ptySize)
							}
						default:
							log.Print("Unknown session request: ", req.Type)
						}

					}
				}()
			case request, ok := <-globalRequestCh:
				if !ok {
					cancel()
					return
				}
				log.Print("Request recieved: ", request.Type)
				if !request.WantReply {
					continue
				}
				request.Reply(false, nil)
			case <-ctx.Done():
				return
			}
		}
	}()

	log.Print("Connection closed: ", conn.Wait())
}
