#ifndef DRM_EGL_H
#define DRM_EGL_H

#include <stdlib.h>
#include <stdint.h>
#include <xf86drm.h>
#include <xf86drmMode.h>
#include <gbm.h>
#include <EGL/egl.h>
#include <GLES2/gl2.h>
#include <drm/drm_mode.h>

#ifdef __cplusplus
extern "C" {
#endif

// 上下文结构体
typedef struct {
    int drm_fd;
    uint32_t connector_id;
    drmModeModeInfo drm_mode;
    drmModeCrtc* saved_crtc;
    
    struct gbm_device* gbm_dev;
    struct gbm_surface* gbm_surf;
    
    EGLDisplay egl_display;
    EGLConfig config;
    EGLContext egl_context;
    EGLSurface egl_surface;
    
    struct gbm_bo* current_bo;
    uint32_t current_fb;
    struct gbm_bo* next_bo;
    uint32_t next_fb;

    int is_offscreen;          // 是否无屏幕模式
} DRMEGLContext;

// 初始化函数
DRMEGLContext*  drm_egl_init(int width, int height);
void drm_egl_render_frame(DRMEGLContext* ctx);
void drm_egl_cleanup(DRMEGLContext* ctx);

#ifdef __cplusplus
}
#endif

#endif // DRM_EGL_H