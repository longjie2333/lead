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
$env:AUTH_DB_PATH="data/auth.db"
```

## Accounts and devices

The default account is `admin/admin`. Override the password with `ADMIN_PASSWORD`.

APIs:

- `POST /api/login`
  - viewer: `{"username":"admin","password":"admin","source":"viewer"}`
  - android: `{"username":"admin","password":"admin","source":"android","deviceId":"device-1","deviceName":"Pixel"}`
- `GET /api/devices` with `Authorization: Bearer <token>`
- `POST /api/logout` with `Authorization: Bearer <token>`

Tokens do not expire on the server side. Logging out removes the token; otherwise the client can keep using it. Tokens and device bindings are persisted in SQLite at `AUTH_DB_PATH`; device online state is recalculated after restart.

SQLite is managed through Go's `database/sql` package with the `github.com/mattn/go-sqlite3` driver. The server initializes the required tables automatically on startup.

Open a specific device from the viewer with:

```text
http://localhost:8787/viewer?deviceId=device-1
```

The viewer stores its token locally and connects signaling as `/ws?token=<token>&deviceId=<deviceId>`. Android logs in before opening the WebSocket, which binds that device to the account.
