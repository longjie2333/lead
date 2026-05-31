# Lead TURN Server

Go TURN service used as the WebRTC relay fallback for the remote assist app.

## Run

```powershell
$env:TURN_PUBLIC_IP="10.10.10.4"
$env:TURN_USERNAME="lead"
$env:TURN_CREDENTIAL="leadpass"
go run .
```

The web signaling server should receive matching ICE environment values:

```powershell
$env:TURN_URL="turn:10.10.10.4:3478"
$env:TURN_USERNAME="lead"
$env:TURN_CREDENTIAL="leadpass"
```

Optional relay port range:

```powershell
$env:TURN_MIN_PORT="50000"
$env:TURN_MAX_PORT="50100"
```
