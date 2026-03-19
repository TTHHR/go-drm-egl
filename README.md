# go-drm-egl

`go-drm-egl` 是一个为 Go 语言开发的轻量级 CGO 包装库，旨在解决嵌入式 Linux（如 Rockchip RK3568/RK3588, Raspberry Pi 等）在没有 X11 或 Wayland 桌面环境下的 GPU 加速渲染问题。通过直接操作 DRM/KMS 和 EGL，实现真正的“开机即渲染”。

## ✨ 特性

*   **零桌面依赖**：直接在 Linux TTY 控制台下工作。
*   **硬件加速**：支持通过 EGL/GLESv2 调用 GPU。
*   **双缓冲机制**：内置 DRM Page Flip，有效防止画面撕裂。
*   **跨架构支持**：适配 amd64 (Intel/AMD 显卡) 和 arm64 (嵌入式芯片)。

## 🛠️ 环境准备

### 1. 系统依赖
在编译和运行之前，请确保你的系统中安装了以下开发库：

**Ubuntu/Debian:**

sudo apt-get update
sudo apt-get install libdrm-dev libgbm-dev libegl1-mesa-dev libgles2-mesa-dev pkg-config


### 2. 硬件与权限
*   **显卡支持**：需要支持 DRM/KMS 驱动的显卡（大部分现代显卡和嵌入式 SoC 均支持）。
*   **关闭桌面服务**：DRM 需要独占显卡控制权。如果系统开启了图形界面（Gnome/KDE/X11），请先关闭：

    sudo systemctl stop gdm    # 或 lightdm, sddm

*   **权限说明**：运行程序通常需要 root 权限，或者将用户加入 `video` 组：

    sudo usermod -aG video $USER


## 🚀 快速开始

### 安装

go get github.com/tthhr/go-drm-egl


### 运行示例代码
本项目提供了两个测试程序：一个简单的三角形渲染（triangle_test）和一个imgui渲染。

本地运行：


进入测试目录

cd examples/triangle_test

运行 (确保你在 TTY 终端下，而不是 SSH 或 远程桌面)

go run main.go


## 📺 运行效果
> **注意**：以下截图为 PC虚拟机

| 3D 三角形测试 |
| :---: |
| ![Triangle](./examples/triangle_test/triangle.jpg) |

## 📂 项目结构说明
*   `/drm`：核心 CGO 逻辑，包含 DRM 初始化、GBM 缓冲管理及 EGL 上下文创建。
*   `/examples`：示例程序，展示如何初始化设备并编写渲染循环。

## 🏗️ 交叉编译 (以 ARM64 为例)
如果你在 x86 电脑上为嵌入式设备编译，请确保已安装 `aarch64-linux-gnu-gcc`：

```
CGO_ENABLED=1 \
GOOS=linux \
GOARCH=arm64 \
CC=aarch64-linux-gnu-gcc \
go build -o drm_demo_arm64 ./examples/triangle_test
```

如果你使用 Buildroot 或 Yocto 生成的 SDK 进行交叉编译，请参考以下脚本。这能确保 CGO 正确链接到 ARM64 架构的底层 libdrm 和 gbm 库。
```
#!/bin/bash

# === 1. 路径配置 (请根据你的环境修改) ===
SDK_PATH=/home/harry/rock/atk_rkx_linux/buildroot/output/rockchip_rk3568/host
SYSROOT=${SDK_PATH}/aarch64-buildroot-linux-gnu/sysroot

# === 2. Go 交叉编译设置 ===
export GOOS=linux
export GOARCH=arm64
export CGO_ENABLED=1

# 指向 SDK 中的交叉编译器
export CC=${SDK_PATH}/bin/aarch64-buildroot-linux-gnu-gcc

# === 3. 核心：设置 CGO 编译与链接参数 ===
# --sysroot 确保编译器在正确的目录下寻找头文件和库
export CGO_CFLAGS="--sysroot=${SYSROOT} -I${SYSROOT}/usr/include -I${SYSROOT}/usr/include/drm"
export CGO_LDFLAGS="--sysroot=${SYSROOT} -L${SYSROOT}/usr/lib -L${SYSROOT}/lib"

# 如果你使用了 pkg-config，请务必指定正确的路径防止搜到宿主机库
export PKG_CONFIG_PATH=${SYSROOT}/usr/lib/pkgconfig
export PKG_CONFIG_SYSROOT_DIR=${SYSROOT}

# === 4. 执行编译 ===
echo "正在编译 ARM64 版本的 DRM 渲染器..."
go build -o triangle_test ./examples/triangle_test

```

## ⚖️ 开源协议
MIT License