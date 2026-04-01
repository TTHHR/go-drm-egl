package main

/*
#cgo pkg-config: libdrm gbm egl glesv2
#cgo LDFLAGS: -lstdc++
#cgo CXXFLAGS: -std=c++17
#include "imgui_wrapper.h"
*/
import "C"

import (
	"fmt"
	"runtime"
	"time"

	"github.com/tthhr/go-drm-egl/drm"
)

func main() {
	runtime.LockOSThread()

	ctx, err := drm.Init(0, 0)
	if err != nil {
		panic(fmt.Sprintf("drm.Init failed: %v", err))
	}
	defer ctx.Cleanup()

	if !drm.MakeCurrent(ctx) {
		panic("drm.MakeCurrent failed")
	}

	drm.ViewPort(0, 0, ctx.Width(), ctx.Height())
	C.go_imgui_init(C.int(ctx.Width()), C.int(ctx.Height()))
	defer C.go_imgui_shutdown()

	frame := 0
	lastTime := time.Now()

	fmt.Println("imgui_test started (ESC/Ctrl+C to quit)")

	for frame < 10000 {
		now := time.Now()
		dt := float32(now.Sub(lastTime).Seconds())
		if dt <= 0 {
			dt = 1.0 / 60.0
		}
		lastTime = now

		C.go_imgui_new_frame(C.int(ctx.Width()), C.int(ctx.Height()), C.float(dt))
		C.go_imgui_build_ui(C.int(frame))

		drm.ClearColor(0.06, 0.06, 0.07, 1.0)
		drm.Clear(drm.COLOR_BUFFER_BIT | drm.DEPTH_BUFFER_BIT)

		C.go_imgui_render()
		drm.RenderFrame(ctx)

		frame++
		if frame%60 == 0 {
			fmt.Printf("rendered %d frames\n", frame)
		}

		time.Sleep(16 * time.Millisecond)
	}

	fmt.Println("imgui_test done")
}
