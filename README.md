# single-rcon

WIP: currently I expect to use this only by myself.
You can use this but there are few document.



The simple way to take control remote host of who needs your assist.

- Work with single binary
- Configurable with simple file

`single-rcon-server`: Simple SSH server only accepts reverse tunnel.

`single-rcon-client`: SSH Server but listen only at remote server.

## How it works

1. `single-rcon-client` connect to `single-rcon-server` with public key authentication.
2. `single-rcon-client` register reverse proxy to `single-rcon-server`.
3. `single-rcon-server` listens only at confugured address:port and proxy connection to `single-rcon-client`.
4. `single-rcon-client` accepts connection and authenticate with public key.

### Security

To connect remote server, both of them.
- Access to client ports of `single-rcon-server`
- A Private key matches the registered key in `single-rcon-client`

## How to use

### Prerequisite: Setup Server
1. Prepare server with single exposed port
2. Download and save `single-rcon-server` and `configs/server-config.yaml`
3. Edit `server-config.yaml`
4. Run single-rcon-server

Warning: I recommend expose only main listening port. Exposing client ports may be safe because of authentication by client, but hiding ports is highly recommend to mitigate any attacks.

### Run Client
Server Owner / assistant
1. Download and save `configs/client-config.yaml`
2. Edit `client-config.yaml` and put your server information
3. Pass `client-config.yaml` to who take your assist

Who take your assist
1. Download and save `single-rcon-client` and `client-config.yaml`(which given from assistant).
2. Run `sudo single-rcon-client`

### End assist
Run `sudo single-rcon-client uninstall`
