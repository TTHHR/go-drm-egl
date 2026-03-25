package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/tthhr/go-drm-egl/drm"
)

const (
	vertexShaderSource = `#version 300 es
layout (location = 0) in vec3 aPos;
layout (location = 1) in vec2 aTexCoord;

out vec2 TexCoord;

void main() {
	gl_Position = vec4(aPos, 1.0);
	TexCoord = aTexCoord;
}`

	fragmentShaderSource = `#version 300 es
precision mediump float;

in vec2 TexCoord;
out vec4 FragColor;

uniform sampler2D uTexture;

void main() {
	FragColor = texture(uTexture, TexCoord);
}`
)

func main() {
	runtime.LockOSThread()

	ctx, err := drm.Init(0, 0)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize DRM/EGL: %v", err))
	}
	defer ctx.Cleanup()

	fmt.Printf("Initialized DRM/EGL: %dx%d\n", ctx.Width(), ctx.Height())

	if !drm.MakeCurrent(ctx) {
		panic("failed to make EGL context current")
	}
	drm.ViewPort(0, 0, ctx.Width(), ctx.Height())

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
	defer drm.DeleteProgram(shaderProgram)
	defer drm.DeleteShader(vertexShader)
	defer drm.DeleteShader(fragmentShader)

	vertices := []float32{
		-0.8, -0.8, 0.0, 0.0, 0.0,
		0.8, -0.8, 0.0, 1.0, 0.0,
		0.8, 0.8, 0.0, 1.0, 1.0,
		-0.8, -0.8, 0.0, 0.0, 0.0,
		0.8, 0.8, 0.0, 1.0, 1.0,
		-0.8, 0.8, 0.0, 0.0, 1.0,
	}

	vbo := drm.GenBuffers(1)
	vao := drm.GenVertexArrays(1)

	drm.BindVertexArray(vao)
	drm.BindBuffer(drm.ARRAY_BUFFER, vbo)
	drm.BufferData(drm.ARRAY_BUFFER, vertices, drm.STATIC_DRAW)
	drm.VertexAttribPointer(0, 3, drm.FLOAT, false, 5*4, 0)
	drm.EnableVertexAttribArray(0)
	drm.VertexAttribPointer(1, 2, drm.FLOAT, false, 5*4, 3*4)
	drm.EnableVertexAttribArray(1)
	drm.BindBuffer(drm.ARRAY_BUFFER, 0)
	drm.BindVertexArray(0)

	textureData := []uint8{
		255, 0, 0, 255, 0, 255, 0, 255,
		0, 0, 255, 255, 255, 255, 0, 255,
	}

	texture := drm.GenTextures(1)

	drm.ActiveTexture(drm.TEXTURE0)
	drm.BindTexture(drm.TEXTURE_2D, texture)
	drm.TexParameteri(drm.TEXTURE_2D, drm.TEXTURE_WRAP_S, int32(drm.CLAMP_TO_EDGE))
	drm.TexParameteri(drm.TEXTURE_2D, drm.TEXTURE_WRAP_T, int32(drm.CLAMP_TO_EDGE))
	drm.TexParameteri(drm.TEXTURE_2D, drm.TEXTURE_MIN_FILTER, int32(drm.LINEAR))
	drm.TexParameteri(drm.TEXTURE_2D, drm.TEXTURE_MAG_FILTER, int32(drm.LINEAR))
	drm.TexImage2D(drm.TEXTURE_2D, 0, drm.RGBA, 2, 2, 0, drm.RGBA, drm.UNSIGNED_BYTE, drm.Ptr(textureData))
	drm.BindTexture(drm.TEXTURE_2D, 0)

	textureLocation := drm.GetUniformLocation(shaderProgram, "uTexture")

	renderUntil := time.Now().Add(3 * time.Second)
	for time.Now().Before(renderUntil) {
		drm.ClearColor(0.1, 0.1, 0.1, 1.0)
		drm.Clear(drm.COLOR_BUFFER_BIT)

		drm.UseProgram(shaderProgram)
		drm.ActiveTexture(drm.TEXTURE0)
		drm.BindTexture(drm.TEXTURE_2D, texture)
		drm.Uniform1i(textureLocation, 0)

		drm.BindVertexArray(vao)
		drm.DrawArrays(drm.TRIANGLES, 0, 6)

		drm.RenderFrame(ctx)
	}

	drm.BindTexture(drm.TEXTURE_2D, 0)
	drm.DeleteTextures(texture)
	texture = 0

	fmt.Println("Texture rendered and deleted.")
}
