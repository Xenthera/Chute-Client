## Build Requirements

This repo has a Go backend and a Tauri (Rust + web) frontend.

## macOS Requirements

### Go
- Go 1.22+
```
brew install go
```

### Node.js
- Node.js 20+ (npm)
```
brew install node@20
```

### Rust
- Rust (rustup + cargo)
```
curl https://sh.rustup.rs -sSf | sh
source ~/.cargo/env
```

### System Packages
- Xcode Command Line Tools:
```
xcode-select --install
```

## Windows Requirements

### Go
- Go 1.22+
```
choco install -y golang
```

### Node.js
- Node.js 20+ (npm)
```
choco install -y nodejs-lts
```

### Rust
- Rust (rustup + cargo)
```
curl https://sh.rustup.rs -sSf | sh
```

### System Packages
- Visual Studio Build Tools (C++ workload)
- WebView2 Runtime (usually already installed on Windows 10/11)

## Linux (Debian/Ubuntu) Requirements

### Go
- Go 1.22+
```
sudo apt update
sudo apt install -y golang
```

### Node.js
- Node.js 20+ (npm)
```
sudo apt update
sudo apt install -y nodejs npm
```

### Rust
- Rust (rustup + cargo)
```
curl https://sh.rustup.rs -sSf | sh
source ~/.cargo/env
```

### System Packages
```
sudo apt update
sudo apt install -y \
  build-essential \
  pkg-config \
  libgtk-3-dev \
  libwebkit2gtk-4.0-dev \
  libayatana-appindicator3-dev \
  librsvg2-dev
```

## Linux (Fedora) Requirements

### Go
- Go 1.22+
```
sudo dnf install -y golang
```

### Node.js
- Node.js 20+ (npm)
```
sudo dnf install -y nodejs npm
```

### Rust
- Rust (rustup + cargo)
```
curl https://sh.rustup.rs -sSf | sh
source ~/.cargo/env
```

### System Packages
```
sudo dnf install -y \
  gcc \
  gcc-c++ \
  make \
  pkgconf-pkg-config \
  gtk3-devel \
  webkit2gtk4.0-devel \
  libappindicator-gtk3-devel \
  librsvg2-devel
```

## Linux (Arch) Requirements

### Go
- Go 1.22+
```
sudo pacman -S --noconfirm go
```

### Node.js
- Node.js 20+ (npm)
```
sudo pacman -S --noconfirm nodejs npm
```

### Rust
- Rust (rustup + cargo)
```
curl https://sh.rustup.rs -sSf | sh
source ~/.cargo/env
```

### System Packages
```
sudo pacman -S --noconfirm \
  base-devel \
  pkgconf \
  gtk3 \
  webkit2gtk \
  libappindicator-gtk3 \
  librsvg
```

### Build
Backend:
```
cd backend
go build ./...
```

Frontend (Tauri):
```
cd frontend
npm install
npm run build
```

