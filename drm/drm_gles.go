package drm

/*
#cgo pkg-config: libdrm gbm egl glesv2
#cgo LDFLAGS: -ldrm -lgbm -lEGL -lGLESv2
#include <GLES2/gl2.h>
#include <GLES2/gl2ext.h>
#include <GLES3/gl3.h>
#include <stdlib.h>
*/
import "C"
import (
	"unsafe"
)
// OpenGL ES 常量
const (
	// 着色器类型
	VERTEX_SHADER   = 0x8B31
	FRAGMENT_SHADER = 0x8B30
	
	// 缓冲对象
	ARRAY_BUFFER        = 0x8892
	ELEMENT_ARRAY_BUFFER = 0x8893
	STATIC_DRAW         = 0x88E4
	DYNAMIC_DRAW        = 0x88E8
	
	// 数据类型
	FLOAT          = 0x1406
	UNSIGNED_BYTE  = 0x1401
	UNSIGNED_INT   = 0x1405
	UNSIGNED_SHORT = 0x1403
	
	// 纹理参数
	TEXTURE0       = 0x84C0
	TEXTURE1       = 0x84C1
	TEXTURE_2D      = 0x0DE1
	TEXTURE_WRAP_S  = 0x2802
	TEXTURE_WRAP_T  = 0x2803
	TEXTURE_MIN_FILTER = 0x2801
	TEXTURE_MAG_FILTER = 0x2800
	CLAMP_TO_EDGE   = 0x812F
	LINEAR          = 0x2601
	LINEAR_MIPMAP_LINEAR = 0x2703
	RGB            = 0x1907
	RGBA            = 0x1908
	
	// 渲染状态
	DEPTH_TEST      = 0x0B71
	BLEND           = 0x0BE2
	SRC_ALPHA       = 0x0302
	ONE_MINUS_SRC_ALPHA = 0x0303
	LESS            = 0x0201
	LEQUAL          = 0x0203
	FUNC_ADD        = 0x8006
	COLOR_BUFFER_BIT  = 0x00004000
	DEPTH_BUFFER_BIT  = 0x00000100
	TRIANGLES       = 0x0004

	FRAMEBUFFER     = 0x8D40
	RENDERBUFFER    = 0x8D41
	DEPTH_COMPONENT  = 0x1902
	DEPTH_ATTACHMENT = 0x8D00
	FRAMEBUFFER_COMPLETE = 0x8CD5
	COLOR_ATTACHMENT0    =0x8CE0
	DEPTH_COMPONENT16  = 0x81A5
	FRAMEBUFFER_UNSUPPORTED   =   0x8CDD
	FRAMEBUFFER_INCOMPLETE_ATTACHMENT = 0x8CD6
	FRAMEBUFFER_INCOMPLETE_MISSING_ATTACHMENT = 0x8CD7
	FRAMEBUFFER_INCOMPLETE_DIMENSIONS = 0x8CD9
	RGBA16F = 0x881A
	RGB16F   =    0x881B
	
	// 着色器状态
	COMPILE_STATUS  = 0x8B81
	LINK_STATUS     = 0x8B82
	INFO_LOG_LENGTH = 0x8B84
)
// 顶点缓冲对象相关
func GenBuffers(n int32) uint32 {
    var buffer uint32
    C.glGenBuffers(C.GLsizei(n), (*C.GLuint)(&buffer))
    return buffer
}

func BindBuffer(target uint32, buffer uint32) {
    C.glBindBuffer(C.GLenum(target), C.GLuint(buffer))
}

func BufferData(target uint32, data []float32, usage uint32) {
    C.glBufferData(C.GLenum(target), C.GLsizeiptr(len(data)*4), unsafe.Pointer(&data[0]), C.GLenum(usage))
}

// 着色器相关
func CreateShader(shaderType uint32) uint32 {
    return uint32(C.glCreateShader(C.GLenum(shaderType)))
}

func ShaderSource(shader uint32, source string) {
    csource := C.CString(source)
    defer C.free(unsafe.Pointer(csource))
    C.glShaderSource(C.GLuint(shader), 1, &csource, nil)
}

func CompileShader(shader uint32) {
    C.glCompileShader(C.GLuint(shader))
}

func CreateProgram() uint32 {
    return uint32(C.glCreateProgram())
}

func AttachShader(program, shader uint32) {
    C.glAttachShader(C.GLuint(program), C.GLuint(shader))
}

func LinkProgram(program uint32) {
    C.glLinkProgram(C.GLuint(program))
}

func UseProgram(program uint32) {
    C.glUseProgram(C.GLuint(program))
}

// 顶点数组对象
func GenVertexArrays(n int32) uint32 {
    var array uint32
    C.glGenVertexArrays(C.GLsizei(n), (*C.GLuint)(&array))
    return array
}

func BindVertexArray(array uint32) {
    C.glBindVertexArray(C.GLuint(array))
}

func VertexAttribPointer(index uint32, size int32, xtype uint32, normalized bool, stride int32, offset uintptr) {
    norm := C.GLboolean(0)
    if normalized {
        norm = 1
    }
    C.glVertexAttribPointer(C.GLuint(index), C.GLint(size), C.GLenum(xtype), norm, C.GLsizei(stride), unsafe.Pointer(offset))
}

func EnableVertexAttribArray(index uint32) {
    C.glEnableVertexAttribArray(C.GLuint(index))
}

// 绘制
func DrawArrays(mode uint32, first, count int32) {
    C.glDrawArrays(C.GLenum(mode), C.GLint(first), C.GLsizei(count))
}
func GetError() uint32 {
	return uint32(C.glGetError())
}
// 辅助函数：创建指针
func Ptr(data interface{}) unsafe.Pointer {
	switch v := data.(type) {
	case []float32:
		if len(v) > 0 {
			return unsafe.Pointer(&v[0])
		}
	case []uint8:
		if len(v) > 0 {
			return unsafe.Pointer(&v[0])
		}
	}
	return unsafe.Pointer(nil)
}

func PtrOffset(offset int) unsafe.Pointer {
	return unsafe.Pointer(uintptr(offset))
}

// 字符串辅助函数
func Str(str string) *uint8 {
	cstr := C.CString(str + "\x00")
	return (*uint8)(unsafe.Pointer(cstr))
}

func BlendEquation(mode uint32) {
	C.glBlendEquation(C.GLenum(mode))
}

func BlendFunc(sfactor, dfactor uint32) {
	C.glBlendFunc(C.GLenum(sfactor), C.GLenum(dfactor))
}

func DepthMask(flag bool) {
	mask := C.GLboolean(0)
	if flag {
		mask = 1
	}
	C.glDepthMask(mask)
}
// 新增：状态管理函数
func Enable(cap uint32) {
	C.glEnable(C.GLenum(cap))
}

func Disable(cap uint32) {
	C.glDisable(C.GLenum(cap))
}

func DepthFunc(fn uint32) {
	C.glDepthFunc(C.GLenum(fn))
}
// 新增：纹理相关函数
func GenTextures(n int32) uint32 {
	var texture uint32
	C.glGenTextures(C.GLsizei(n), (*C.GLuint)(&texture))
	return texture
}

func BindTexture(target uint32, texture uint32) {
	C.glBindTexture(C.GLenum(target), C.GLuint(texture))
}

func TexParameteri(target, pname uint32, param int32) {
	C.glTexParameteri(C.GLenum(target), C.GLenum(pname), C.GLint(param))
}

func TexImage2D(target uint32, level int32, internalFormat uint32, width, height int32, 
	border int32, format, xtype uint32, data unsafe.Pointer) {
	C.glTexImage2D(C.GLenum(target), C.GLint(level), C.GLint(internalFormat), 
		C.GLsizei(width), C.GLsizei(height), C.GLint(border), 
		C.GLenum(format), C.GLenum(xtype), data)
}

func GenerateMipmap(target uint32) {
	C.glGenerateMipmap(C.GLenum(target))
}

func ActiveTexture(texture uint32) {
	C.glActiveTexture(C.GLenum(texture))
}

// 新增：Uniform相关函数
func GetUniformLocation(program uint32, name string) int32 {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	return int32(C.glGetUniformLocation(C.GLuint(program), cname))
}

func GetAttribLocation(program uint32, name string) int32 {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	return int32(C.glGetAttribLocation(C.GLuint(program), cname))
}

func Uniform1i(location int32, v int32) {
	C.glUniform1i(C.GLint(location), C.GLint(v))
}

func Uniform1f(location int32, v float32) {
	C.glUniform1f(C.GLint(location), C.GLfloat(v))
}

func Uniform3f(location int32, v0, v1, v2 float32) {
	C.glUniform3f(C.GLint(location), C.GLfloat(v0), C.GLfloat(v1), C.GLfloat(v2))
}

func Uniform4f(location int32, v0, v1, v2, v3 float32) {
	C.glUniform4f(C.GLint(location), C.GLfloat(v0), C.GLfloat(v1), C.GLfloat(v2), C.GLfloat(v3))
}

func UniformMatrix4fv(location int32, count int32, transpose bool, value *float32) {
	trans := C.GLboolean(0)
	if transpose {
		trans = 1
	}
	C.glUniformMatrix4fv(C.GLint(location), C.GLsizei(count), trans, (*C.GLfloat)(value))
}
func GetShaderiv(shader uint32, pname uint32, params *int32) {
	C.glGetShaderiv(C.GLuint(shader), C.GLenum(pname), (*C.GLint)(params))
}

func GetProgramiv(program uint32, pname uint32, params *int32) {
	C.glGetProgramiv(C.GLuint(program), C.GLenum(pname), (*C.GLint)(params))
}

func GetProgramInfoLog(program uint32, bufSize int32, length *int32, infoLog *byte) {
	C.glGetProgramInfoLog(C.GLuint(program), C.GLsizei(bufSize), (*C.GLsizei)(length), (*C.GLchar)(unsafe.Pointer(infoLog)))
}
func GetShaderInfoLog(shader uint32, bufSize int32, length *int32, infoLog *byte) {
	C.glGetShaderInfoLog(C.GLuint(shader), C.GLsizei(bufSize), (*C.GLsizei)(length), (*C.GLchar)(unsafe.Pointer(infoLog)))
}
func DeleteShader(shader uint32) {
	C.glDeleteShader(C.GLuint(shader))
}

func DeleteProgram(program uint32) {
	C.glDeleteProgram(C.GLuint(program))
}

func GenRenderbuffers(n int32) uint32 {
	var renderbuffer uint32
	C.glGenRenderbuffers(C.GLsizei(n), (*C.GLuint)(&renderbuffer))
	return renderbuffer
}

func BindRenderbuffer(target uint32, renderbuffer uint32) {
	C.glBindRenderbuffer(C.GLenum(target), C.GLuint(renderbuffer))
}
func GenFramebuffers(n int32) uint32 {
	var framebuffer uint32
	C.glGenFramebuffers(C.GLsizei(n), (*C.GLuint)(&framebuffer))
	return framebuffer
}
func BindFramebuffer(target uint32, framebuffer uint32) {
	C.glBindFramebuffer(C.GLenum(target), C.GLuint(framebuffer))
}
func RenderbufferStorage(target uint32, internalFormat uint32, width, height int32) {
	C.glRenderbufferStorage(C.GLenum(target), C.GLenum(internalFormat), C.GLsizei(width), C.GLsizei(height))
}
func FramebufferRenderbuffer(target, attachment, renderbuffertarget uint32, renderbuffer uint32) {
	C.glFramebufferRenderbuffer(C.GLenum(target), C.GLenum(attachment), C.GLenum(renderbuffertarget), C.GLuint(renderbuffer))
}
func FramebufferTexture2D(target, attachment, textarget uint32, texture uint32, level int32) {
	C.glFramebufferTexture2D(C.GLenum(target), C.GLenum(attachment), C.GLenum(textarget), C.GLuint(texture), C.GLint(level))
}
func CheckFramebufferStatus(target uint32) uint32 {
	return uint32(C.glCheckFramebufferStatus(C.GLenum(target)))
}