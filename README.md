# Lead 远程协助项目

一个完整的远程屏幕协助解决方案，旨在通过 WebRTC 技术实现 Android 设备的实时屏幕共享与远程控制。项目由 Go 后端服务、Android 原生应用和 Web 端三部分组成，后端集成了 WebSocket 信令服务和 TURN 中继服务，支持 P2P 直连和服务端中转两种传输模式，确保在各种网络环境下都能稳定工作。

整个项目基于 WebRTC 实现了低延迟的视频传输，后端使用 Gin 框架提供 RESTful API 和 WebSocket 信令通道，集成 Pion TURN 库提供 NAT 穿透能力，使用 SQLite 持久化用户和设备数据。Android 端采用 Kotlin + Jetpack Compose 构建现代化 UI，通过 MediaProjection 捕获屏幕内容，使用 Accessibility Service 实现远程控制，利用 WebRTC Android SDK 完成视频编码和传输。Web 端基于 Vue3 + Vite + Pinia + Tailwind CSS 开发，提供设备列表、远程控制和用户管理页面；生产构建产物写入 `server/public`，由 Go 服务直接托管。

## 技术选型

| 项目             | 对应目录 | 技术栈                                      |
| ---------------- | -------- | ------------------------------------------- |
| **后端服务**     | server/  | Go + Gin + WebSocket + SQLite3 + Pion Turn  |
| **Android 应用** | app/     | Kotlin + Miuix + WebRTC + OkHttp            |
| **Web 端**       | server-web/ | Vue3 + Vite + Pinia + Tailwind CSS         |

## 快速开始

### 前置要求

- **后端服务**
    - Go 1.23.3 或更高版本
- **Android 应用**
    - JDK 17
    - Android SDK API 26-36
- **Web 端**
    - Node.js 18.x 或更高版本
    - npm

### 本地运行

**后端服务**

进入后端目录并启动服务：

```powershell
cd server

# 本地开发 (使用回环地址)
$env:TURN_PUBLIC_IP="127.0.0.1"
$env:ADMIN_PASSWORD="your-admin-password"
go run ./cmd/server
```

局域网真机测试时，需要设置电脑的局域网 IP 地址：

```powershell
# 替换为你的局域网 IP
$env:TURN_PUBLIC_IP="192.168.1.20"
$env:TURN_USERNAME="lead"
$env:TURN_CREDENTIAL="leadpass"
$env:ADMIN_PASSWORD="your-admin-password"
go run ./cmd/server
```

服务启动后：
- HTTP API 和 WebSocket 信令：http://localhost:8787/
- WebSocket 信令：ws://localhost:8787/ws
- Android 连接地址：ws://192.168.1.20:8787/ws (局域网测试)

更多配置选项参见 [server/README.md](server/README.md)。

**Android 应用**

使用 Android Studio 打开 `app/` 目录，或使用命令行构建：

```bash
cd app

# 调试构建
./gradlew assembleDebug

# Release 构建需要配置签名，参见下文"构建配置"部分
./gradlew assembleRelease
```

构建前需要配置后端服务地址，编辑 `app/local.properties` 或设置环境变量：

```properties
# local.properties
screenShareServerUrl=ws://192.168.1.20:8787/ws
```

或通过 Gradle 属性：

```bash
./gradlew assembleDebug -PscreenShareServerUrl=ws://192.168.1.20:8787/ws
```

**Web 端**

进入 Web 端目录并安装依赖：

```bash
cd server-web
npm ci

# 开发 Web 端
npm run dev

# 构建到 server/public
npm run build
```

开发服务默认运行在 http://localhost:5173/，并把 `/api` 和 `/ws` 代理到本地后端 `127.0.0.1:8787`。生产构建产物会写入 `server/public`，线上由 Go 服务在 http://localhost:8787/ 直接托管。

### 后端部署

**使用 Docker Compose (推荐)**

最简单的部署方式是使用 Docker Compose：

```bash
cd server-web
npm ci
npm run build

cd server
docker-compose up -d --build
```

配置文件位于 `server/docker-compose.yml`，默认配置：
- HTTP 服务端口：8787
- TURN UDP 端口：3478
- TURN relay 端口范围：50000-50100
- 数据持久化：./data 目录

自定义配置时编辑 `docker-compose.yml` 中的环境变量，或创建 `.env` 文件：

```bash
# .env
TURN_PUBLIC_IP=your.public.ip
TURN_USERNAME=custom_user
TURN_CREDENTIAL=custom_pass
ADMIN_PASSWORD=secure_password
```

**使用 Docker 镜像**

直接运行 Docker 容器：

```bash
docker run -d \
  --name lead-server \
  -p 8787:8787 \
  -p 3478:3478/udp \
  -p 50000-50100:50000-50100/udp \
  -e TURN_PUBLIC_IP=your.public.ip \
  -e ADMIN_PASSWORD=your-password \
  -v ./data:/app/data \
  ghcr.io/longjie2333/lead-server:latest
```

**使用预编译二进制**

从 GitHub Releases 下载对应平台的二进制包：
- Windows: lead-server-x.x.x-windows-amd64.zip
- Linux x64: lead-server-x.x.x-linux-amd64.tar.gz
- Linux ARM64: lead-server-x.x.x-linux-arm64.tar.gz

解压后运行：

```bash
# Linux
export TURN_PUBLIC_IP="your.public.ip"
export ADMIN_PASSWORD="your-password"
./lead-server

# Windows
$env:TURN_PUBLIC_IP="your.public.ip"
$env:ADMIN_PASSWORD="your-password"
.\lead-server.exe
```

**防火墙配置**

生产环境需要放行以下端口：

- TCP 8787: HTTP API 和 WebSocket 信令
- UDP 3478: TURN 入口端口
- UDP 50000-50100: TURN relay 媒体转发端口 (可通过 TURN_MIN_PORT/TURN_MAX_PORT 配置)

### 使用说明

默认管理员用户名和密码均为： `admin` （生产环境建议通过 `ADMIN_PASSWORD` 环境变量修改密码）

## 构建配置

### Android 签名配置

Release 构建需要配置签名，创建 `app/signing.properties` 文件：

```properties
storeFile=release.keystore
storePassword=your-keystore-password
keyAlias=your-key-alias
keyPassword=your-key-password
```

签名文件生成方式：

```bash
keytool -genkey -v \
  -keystore release.keystore \
  -alias your-key-alias \
  -keyalg RSA \
  -keysize 2048 \
  -validity 10000 \
  -storepass your-keystore-password \
  -keypass your-key-password
```

生成后将 `release.keystore` 放在 `app/` 目录下，与 `signing.properties` 同级。

### 后端交叉编译

后端使用 CGO 编译 SQLite，需要对应平台的 C 工具链：

**Linux x64 目标 (在 Windows/Linux 上)**
```bash
# Linux 宿主机
apt-get install build-essential

# Windows 宿主机 (使用 WSL 或 MinGW)
CC=x86_64-linux-gnu-gcc CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o lead-server ./cmd/server
```

**Windows x64 目标 (在 Linux 上)**
```bash
apt-get install gcc-mingw-w64-x86-64
CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build -o lead-server.exe ./cmd/server
```

**Linux ARM64 目标 (在 Linux 上)**
```bash
apt-get install gcc-aarch64-linux-gnu
CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -o lead-server ./cmd/server
```

## CI/CD 说明

项目使用 GitHub Actions 进行自动化构建和发布，包含两个主要 workflow：

|                 | Android 构建流程                                             | 后端构建流程                                                 |
| --------------- | ------------------------------------------------------------ | ------------------------------------------------------------ |
| **对应文件**    | `.github/workflows/android.build.yml`                        | `.github/workflows/backend.build.yml`                        |
| **触发条件**    | - Push 到 dev/main/master 分支<br />- Pull Request 到 dev/main/master 分支<br />- 创建 v* 版本标签<br />- 手动触发 | 与左边相同                                                   |
| **构建目标**    | - APK<br />- AAB                                             | - Web 静态资源<br />- Linux x64<br />- Linux ARM64<br />- Windows x64 |
| **Docker 镜像** | *无*                                                         | 自动构建并推送到 GitHub Container Registry (ghcr.io)：<br />- dev 分支 → `ghcr.io/longjie2333/lead-server:dev`<br/>- 其他分支 → `ghcr.io/longjie2333/lead-server:<branch-name>`<br/>- 版本标签 → `ghcr.io/longjie2333/lead-server:<tag-name>` |
| **自动发布**    | 标签构建时自动创建 GitHub Release 并上传 APK/AAB 文件        | 标签构建时自动创建 GitHub Release，上传包含 Web 端静态资源的服务端二进制包和 SHA256 校验文件。 |

### Android 构建配置

**Github Secrets**

- `ANDROID_KEYSTORE_BASE64`：签名密钥库的 Base64 编码
- `ANDROID_KEYSTORE_PASSWORD`：密钥库密码
- `ANDROID_KEY_ALIAS`：密钥别名
- `ANDROID_KEY_PASSWORD`：密钥密码

**Github Variables**

- `ANDROID_SERVER_URL`：后端服务器地址 (编译时注入到 APK)

**生成 Base64 编码的密钥库：**

```bash
# Linux/macOS
base64 -w 0 release.keystore > release.keystore.base64

# Windows (PowerShell)
[Convert]::ToBase64String([IO.File]::ReadAllBytes("release.keystore")) | Set-Content release.keystore.base64 -NoNewline
```

将 `release.keystore.base64` 文件内容复制到 GitHub Secrets 的 `ANDROID_KEYSTORE_BASE64` 中。

### 后端与 Web 构建配置

后端 workflow 会先调用 `.github/actions/web-build` 执行 `server-web` 的 `npm ci` 和 `npm run build`，生成 `server/public` 后再调用 `.github/actions/go-cross-build` 打包服务端。Docker 镜像构建前也会执行同一个 Web 构建 action，确保镜像内包含最新 Web 端页面。

### 本地测试 CI 构建

**测试 Android 构建**

```bash
cd app
export VERSION_CODE=1
export VERSION_NAME=1.0.0-test
export SCREEN_SHARE_SERVER_URL=ws://test.example.com:8787/ws
./gradlew testReleaseUnitTest lintRelease assembleRelease bundleRelease
```

**测试后端构建**

```bash
cd server-web
npm ci
npm run build

cd server
# 构建 Linux x64
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/lead-server ./cmd/server

# 构建 Windows x64 (需要 MinGW)
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -ldflags="-s -w" -o dist/lead-server.exe ./cmd/server
```

## 项目结构

```
lead/
├── server/                   # Go 后端服务
│   ├── cmd/server/           # 程序入口
│   ├── internal/             # 内部包
│   │   ├── app/              # HTTP 服务装配
│   │   ├── auth/             # 用户鉴权与设备管理
│   │   ├── config/           # 配置加载
│   │   ├── signaling/        # WebSocket 信令转发
│   │   └── turnserver/       # TURN 中继服务
│   ├── public/               # server-web 构建产物，已忽略提交
│   ├── data/                 # SQLite 数据目录
│   ├── Dockerfile            # Docker 镜像构建
│   ├── docker-compose.yml    # Docker Compose 配置
│   └── README.md             # 后端详细文档
├── app/                      # Android 原生应用
│   ├── app/                  # 应用模块
│   │   ├── src/              # 源代码
│   │   └── build.gradle.kts  # 模块构建配置
│   ├── gradle/               # Gradle wrapper
│   ├── signing.properties    # 签名配置 (需自行创建)
│   └── build.gradle.kts      # 项目构建配置
├── server-web/               # Web 端
│   ├── src/                  # 源代码
│   ├── package.json          # npm 配置
│   └── vite.config.js        # Vite 构建配置
└── .github/                  # GitHub Actions
    ├── workflows/            # CI/CD 工作流
    │   ├── android.build.yml # Android 构建
    │   └── backend.build.yml # 后端构建
    └── actions/              # 可复用 actions
        ├── android-build/    # Android 构建封装
        ├── go-cross-build/   # Go 交叉编译封装
        └── web-build/        # Web 端构建封装
```
