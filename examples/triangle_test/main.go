package main

import "github.com/tthhr/go-drm-egl/drm"
import (
	"fmt"
	"math/rand"
	"runtime"
	"time"
)
const (
	vertexShaderSource = `#version 300 es
		in vec3 aPos;
		void main() {
			gl_Position = vec4(aPos.x, aPos.y, aPos.z, 1.0);
		}`

	fragmentShaderSource = `#version 300 es
		precision mediump float;
		out vec4 FragColor;
		void main() {
			FragColor = vec4(1.0, 0.5, 0.2, 1.0); // 橙色三角形
		}`
)
func main() {
    // 初始化 DRM/EGL 上下文
    ctx, err := drm.Init(0,0) //0 0表示使用屏幕默认大小
    if err != nil {
        panic(fmt.Sprintf("Failed to initialize DRM/EGL: %v", err))
    }
    defer ctx.Cleanup()
    
    fmt.Printf("Initialized DRM/EGL: %dx%d\n", ctx.Width(), ctx.Height())
    
    runtime.LockOSThread()
	

    rand.Seed(time.Now().UnixNano())
    drm.MakeCurrent(ctx)
    drm.ViewPort(0, 0, ctx.Width(), ctx.Height())

   // 创建着色器程序
	vertexShader := drm.CreateShader(drm.VERTEX_SHADER)
	drm.ShaderSource(vertexShader, vertexShaderSource)
	drm.CompileShader(vertexShader)

	fragmentShader := drm.CreateShader(drm.FRAGMENT_SHADER)
	drm.ShaderSource(fragmentShader, fragmentShaderSource)
	drm.CompileShader(fragmentShader)

	shaderProgram := drm.CreateProgram()
	drm.AttachShader(shaderProgram, vertexShader)
	drm.AttachShader(shaderProgram, fragmentShader)
	drm.LinkProgram(shaderProgram)

	// 三角形顶点数据 (NDC坐标)
	vertices := []float32{
		-0.5, -0.5, 0.0, // 左下
		0.5, -0.5, 0.0,  // 右下
		0.0, 0.5, 0.0,   // 顶部
	}

	// 创建VBO和VAO
	VBO := drm.GenBuffers(1)
	VAO := drm.GenVertexArrays(1)

	drm.BindVertexArray(VAO)
	drm.BindBuffer(drm.ARRAY_BUFFER, VBO)
	drm.BufferData(drm.ARRAY_BUFFER, vertices, drm.STATIC_DRAW)

	// 设置顶点属性指针
	drm.VertexAttribPointer(0, 3, drm.FLOAT, false, 3 * 4, 0)
	drm.EnableVertexAttribArray(0)

	drm.BindBuffer(drm.ARRAY_BUFFER, 0)
	drm.BindVertexArray(0)
    fps:=0
    start := time.Now()
	// 渲染循环
	for {
		

		// 清除屏幕
		drm.ClearColor(0.2, 0.3, 0.3, 1.0)
		drm.Clear(drm.COLOR_BUFFER_BIT)

		// 绘制三角形
		drm.UseProgram(shaderProgram)
		drm.BindVertexArray(VAO)
		drm.DrawArrays(drm.TRIANGLES, 0, 3)

		// 提交帧
		drm.RenderFrame(ctx)
        fps++
		// 控制帧率
		elapsed := time.Since(start)
		if elapsed >= time.Second {
			fmt.Printf("FPS: %d\n", fps)
            fps = 0
            start = time.Now()
            continue
		}
	}
}
