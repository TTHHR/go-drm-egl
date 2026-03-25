package main

import (
	"fmt"
	"math"
	"runtime"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/AllenDang/cimgui-go/impl/opengl3"
	"github.com/tthhr/go-drm-egl/drm"
)

func main() {
	runtime.LockOSThread()

	ctx, err := drm.Init(0, 0)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize DRM/EGL: %v", err))
	}
	defer ctx.Cleanup()

	if !drm.MakeCurrent(ctx) {
		panic("failed to make EGL context current")
	}

	drm.ViewPort(0, 0, ctx.Width(), ctx.Height())
	drm.Enable(drm.BLEND)
	drm.BlendFunc(drm.SRC_ALPHA, drm.ONE_MINUS_SRC_ALPHA)

	imguiCtx := imgui.CreateContext()
	defer imgui.DestroyContext()
	imgui.SetCurrentContext(imguiCtx)
	imgui.StyleColorsDark()

	if !opengl3.InitV("#version 300 es") {
		panic("failed to initialize cimgui-go opengl3 backend")
	}
	defer opengl3.Shutdown()

	fmt.Printf("Initialized DRM/EGL: %dx%d\n", ctx.Width(), ctx.Height())

	showDemo := true
	progress := float32(0)
	frames := 0
	lastTick := time.Now()
	lastFPS := lastTick
	fps := 0

	for {
		now := time.Now()
		delta := now.Sub(lastTick).Seconds()
		lastTick = now
		frames++
		progress = float32(math.Mod(float64(progress)+delta*0.25, 1.0))

		io := imgui.CurrentIO()
		io.SetDisplaySize(imgui.NewVec2(float32(ctx.Width()), float32(ctx.Height())))
		io.SetDeltaTime(float32(delta))

		opengl3.NewFrame()
		imgui.NewFrame()

		imgui.SetNextWindowPos(imgui.NewVec2(24, 24))
		imgui.SetNextWindowSize(imgui.NewVec2(420, 220))
		if imgui.Begin("cimgui-go on DRM/EGL") {
			imgui.Text("Renderer: DRM + EGL + OpenGL ES 3.0")
			imgui.Text(fmt.Sprintf("Framebuffer: %dx%d", ctx.Width(), ctx.Height()))
			imgui.Text(fmt.Sprintf("Frames: %d", frames))
			imgui.Text(fmt.Sprintf("FPS: %d", fps))
			imgui.Separator()
			imgui.Text("This example uses cimgui-go + impl/opengl3")
			imgui.Text("No platform input backend is wired yet.")
			imgui.ProgressBar(progress)
		}
		imgui.End()

		if showDemo {
			imgui.ShowDemoWindowV(&showDemo)
		}

		imgui.Render()

		drm.ClearColor(0.08, 0.10, 0.12, 1.0)
		drm.Clear(drm.COLOR_BUFFER_BIT)
		opengl3.RenderDrawData(imgui.CurrentDrawData())
		drm.RenderFrame(ctx)

		if time.Since(lastFPS) >= time.Second {
			fps = frames
			frames = 0
			lastFPS = time.Now()
			fmt.Printf("FPS: %d\n", fps)
		}
	}
}
