package drm

/*
#cgo pkg-config: libdrm gbm egl glesv2
#cgo LDFLAGS: -ldrm -lgbm -lEGL -lGLESv2
#include "csrc/drm_egl.h"
#include "csrc/drm_egl.c"
*/
import "C"
import (
	"runtime"
	"unsafe"
	"errors"
)

// Context 表示 DRM/EGL 上下文
type Context struct {
	ctx *C.DRMEGLContext
}

// Init 初始化 DRM/EGL 环境
func Init(w,h int) (*Context, error) {
	cCtx := C.drm_egl_init(C.int(w), C.int(h))
	if cCtx == nil {
		return nil, ErrInitializationFailed
	}
	
	ctx := &Context{ctx: cCtx}
	
	// 设置析构函数，确保资源被释放
	runtime.SetFinalizer(ctx, func(c *Context) {
		c.Cleanup()
	})
	
	return ctx, nil
}
func ViewPort(x,y, w, h int) {
    C.glViewport(C.int(x), C.int(y), C.int(w), C.int(h))
}
func ClearColor(r, g, b,a float32) {
    C.glClearColor(C.float(r), C.float(g), C.float(b), C.float(a))
}
func Clear(mask uint32) {
	C.glClear(C.GLbitfield(mask))
}
// RenderFrame 渲染一帧
func RenderFrame(c *Context) {
	C.drm_egl_render_frame(c.ctx)
}

// Cleanup 清理资源
func (c *Context) Cleanup() {
	if c.ctx != nil {
		C.drm_egl_cleanup(c.ctx)
		c.ctx = nil
	}
}

// MakeCurrent 设置当前上下文
func MakeCurrent(c *Context) bool {
	return C.eglMakeCurrent(c.ctx.egl_display, c.ctx.egl_surface, c.ctx.egl_surface, c.ctx.egl_context) != 0
}

// Width 获取显示宽度
func (c *Context) Width() int {
	return int(c.ctx.drm_mode.hdisplay)
}

// Height 获取显示高度
func (c *Context) Height() int {
	return int(c.ctx.drm_mode.vdisplay)
}

// GetProcAddress 获取 OpenGL ES 函数指针
func (c *Context) GetProcAddress(name string) unsafe.Pointer {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	return unsafe.Pointer(C.eglGetProcAddress(cname))
}

// 错误定义
var (
	ErrInitializationFailed = errors.New("failed to initialize DRM/EGL context")
)