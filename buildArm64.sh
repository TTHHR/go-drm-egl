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
export CXX=${SDK_PATH}/bin/aarch64-buildroot-linux-gnu-g++

# === 3. 核心：设置 CGO 编译与链接参数 ===
# --sysroot 确保编译器在正确的目录下寻找头文件和库
export CGO_CFLAGS="--sysroot=${SYSROOT} -I${SYSROOT}/usr/include -I${SYSROOT}/usr/include/drm"
export CGO_CXXFLAGS="--sysroot=${SYSROOT} -I${SYSROOT}/usr/include -I${SYSROOT}/usr/include/drm"
export CGO_LDFLAGS="--sysroot=${SYSROOT} -L${SYSROOT}/usr/lib -L${SYSROOT}/lib"
export CGO_LDFLAGS_ALLOW="-Wl,--whole-archive|-Wl,--no-whole-archive"

# 如果你使用了 pkg-config，请务必指定正确的路径防止搜到宿主机库
export PKG_CONFIG_PATH=${SYSROOT}/usr/lib/pkgconfig
export PKG_CONFIG_SYSROOT_DIR=${SYSROOT}

# === 4. 执行编译 ===
echo "正在编译 ARM64 版本的 DRM 渲染器..."

go build -o triangle_test ./examples/triangle_test

export CGO_CXXFLAGS="--sysroot=${SYSROOT} -I${SYSROOT}/usr/include -I${SYSROOT}/usr/include/drm -std=c++17 -Iexamples/imgui_test -Iexamples/imgui_test/imgui -DIMGUI_IMPL_OPENGL_ES2"
export CGO_LDFLAGS="--sysroot=${SYSROOT} -L${SYSROOT}/usr/lib -L${SYSROOT}/lib -lstdc++"

go build -v -x -o imgui_test ./examples/imgui_test