# Lead Remote Assist Server

Go backend that packages the remote assist static site, WebSocket signaling, and TURN relay fallback into one process.

## Run

```powershell
$env:TURN_PUBLIC_IP="10.10.10.4"
$env:TURN_USERNAME="lead"
$env:TURN_CREDENTIAL="leadpass"
go run .
```

Then open:

http://localhost:8787/viewer

Android should connect to the same backend WebSocket endpoint, for example `ws://10.10.10.4:8787/ws`.

The server serves static files from `server/public` and automatically advertises `turn:<TURN_PUBLIC_IP>:<TURN_PORT>` to WebRTC clients. Override ICE manually with `ICE_SERVERS_JSON` when needed.

Optional relay port range:

```powershell
$env:TURN_MIN_PORT="50000"
$env:TURN_MAX_PORT="50100"
```

Optional HTTP/static settings:

```powershell
$env:PORT="8787"
$env:HTTP_ADDR="0.0.0.0"
```
