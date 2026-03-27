#include "drm_egl.h"
#include <fcntl.h>
#include <unistd.h>
#include <xf86drm.h>
#include <xf86drmMode.h>
#include <gbm.h>
#include <EGL/egl.h>
#include <EGL/eglext.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int page_flipped = 0;
static void drm_page_flip_handler(int fd, unsigned int frame,
                              unsigned int sec, unsigned int usec,
                              void *data)
{
    page_flipped = 1;
}

static void wait_for_page_flip(int drm_fd) {
    fd_set fds;
    FD_ZERO(&fds);
    FD_SET(drm_fd, &fds);
    while (!page_flipped) 
    {
        fd_set rfds = fds;
        int ret = select(drm_fd + 1, &rfds, NULL, NULL, NULL);
        if (ret < 0) break;
        drmEventContext evctx = {};
        evctx.version = DRM_EVENT_CONTEXT_VERSION;
        evctx.page_flip_handler = drm_page_flip_handler;
        drmHandleEvent(drm_fd, &evctx);
    }
    page_flipped = 0;
}

DRMEGLContext* drm_egl_init(int width, int height) {
    DRMEGLContext* ctx = calloc(1, sizeof(DRMEGLContext));
    if (!ctx) return NULL;
    
    ctx->is_offscreen = (width != 0 && height != 0);
    ctx->drm_fd = open("/dev/dri/card0", O_RDWR | O_CLOEXEC);
    if (ctx->drm_fd < 0) {
        perror("open /dev/dri/card0");
        free(ctx);
        return NULL;
    }

    if (!ctx->is_offscreen) {
        // 屏幕渲染模式：查找连接器和模式
        drmModeRes *res = drmModeGetResources(ctx->drm_fd);
        if (!res) {
            perror("drmModeGetResources");
            close(ctx->drm_fd);
            free(ctx);
            return NULL;
        }

        drmModeConnector *conn = NULL;
        for (int i = 0; i < res->count_connectors; ++i) {
            conn = drmModeGetConnector(ctx->drm_fd, res->connectors[i]);
            if (conn && conn->connection == DRM_MODE_CONNECTED && conn->count_modes > 0) {
                ctx->connector_id = conn->connector_id;
                ctx->drm_mode = conn->modes[0];
                break;
            }
            if (conn) {
                drmModeFreeConnector(conn);
                conn = NULL;
            }
        }

        if (!conn) {
            fprintf(stderr, "No connected connector found.\n");
            drmModeFreeResources(res);
            close(ctx->drm_fd);
            free(ctx);
            return NULL;
        }

        drmModeFreeResources(res);

        drmModeEncoder *enc = drmModeGetEncoder(ctx->drm_fd, conn->encoder_id);
        if (!enc) {
            perror("drmModeGetEncoder");
            drmModeFreeConnector(conn);
            close(ctx->drm_fd);
            free(ctx);
            return NULL;
        }

        ctx->saved_crtc = drmModeGetCrtc(ctx->drm_fd, enc->crtc_id);
        drmModeFreeEncoder(enc);
        drmModeFreeConnector(conn);
    }

    // 创建GBM设备
    ctx->gbm_dev = gbm_create_device(ctx->drm_fd);
    if (!ctx->gbm_dev) {
        fprintf(stderr, "Failed to create GBM device.\n");
        if (ctx->saved_crtc) drmModeFreeCrtc(ctx->saved_crtc);
        close(ctx->drm_fd);
        free(ctx);
        return NULL;
    }

    // 确定表面尺寸
    int surface_width = ctx->is_offscreen ? width : ctx->drm_mode.hdisplay;
    int surface_height = ctx->is_offscreen ? height : ctx->drm_mode.vdisplay;

    // 先不创建GBM surface，等待找到合适配置后根据 EGL_NATIVE_VISUAL_ID 创建
    (void)surface_width;
    (void)surface_height;

    // 初始化EGL
    PFNEGLGETPLATFORMDISPLAYEXTPROC get_platform_display =
        (PFNEGLGETPLATFORMDISPLAYEXTPROC)eglGetProcAddress("eglGetPlatformDisplayEXT");
    if (!get_platform_display) {
        fprintf(stderr, "Failed to get eglGetPlatformDisplayEXT\n");
        drm_egl_cleanup(ctx);
        return NULL;
    }

    ctx->egl_display = get_platform_display(EGL_PLATFORM_GBM_MESA, ctx->gbm_dev, NULL);
    if (ctx->egl_display == EGL_NO_DISPLAY) {
        fprintf(stderr, "Failed to get EGL display.\n");
        drm_egl_cleanup(ctx);
        return NULL;
    }

    if (!eglInitialize(ctx->egl_display, NULL, NULL)) {
        fprintf(stderr, "Failed to initialize EGL.\n");
        drm_egl_cleanup(ctx);
        return NULL;
    }

    if (!eglBindAPI(EGL_OPENGL_ES_API)) {
        fprintf(stderr, "Failed to bind OpenGL ES API.\n");
        drm_egl_cleanup(ctx);
        return NULL;
    }

    // 选择EGL配置：优先ES3，回退ES2
    EGLint config_attribs_es3[] = {
        EGL_SURFACE_TYPE, EGL_WINDOW_BIT,
        EGL_RENDERABLE_TYPE, EGL_OPENGL_ES3_BIT,
        EGL_BLUE_SIZE, 8,
        EGL_GREEN_SIZE, 8,
        EGL_RED_SIZE, 8,
        EGL_DEPTH_SIZE, 16,
        EGL_STENCIL_SIZE, 8,
        EGL_SAMPLE_BUFFERS, 1,
        EGL_SAMPLES, 8,
        EGL_NONE
    };

    EGLint config_attribs_es2[] = {
        EGL_SURFACE_TYPE, EGL_WINDOW_BIT,
        EGL_RENDERABLE_TYPE, EGL_OPENGL_ES2_BIT,
        EGL_BLUE_SIZE, 8,
        EGL_GREEN_SIZE, 8,
        EGL_RED_SIZE, 8,
        EGL_DEPTH_SIZE, 16,
        EGL_STENCIL_SIZE, 8,
        EGL_SAMPLE_BUFFERS, 1,
        EGL_SAMPLES, 8,
        EGL_NONE
    };

    EGLint config_attribs_minimal_es3[] = {
        EGL_SURFACE_TYPE, EGL_WINDOW_BIT,
        EGL_RENDERABLE_TYPE, EGL_OPENGL_ES3_BIT,
        EGL_NONE
    };

    EGLint config_attribs_minimal_es2[] = {
        EGL_SURFACE_TYPE, EGL_WINDOW_BIT,
        EGL_RENDERABLE_TYPE, EGL_OPENGL_ES2_BIT,
        EGL_NONE
    };

    EGLConfig configs[32];
    EGLint num_configs = 0;
    int use_es3 = 1;
    EGLint *try_attrs[] = {
        config_attribs_es3,
        config_attribs_es2,
        config_attribs_minimal_es3,
        config_attribs_minimal_es2,
    };
    int try_versions[] = {3, 2, 3, 2};

    for (int i = 0; i < 4; i++) {
        if (!eglChooseConfig(ctx->egl_display, try_attrs[i], configs, 32, &num_configs) || num_configs == 0) {
            fprintf(stderr, "DEBUG eglChooseConfig attempt %d -> num_configs=%d\n", i, num_configs);
            continue;
        }

        use_es3 = (try_versions[i] == 3);
        EGLConfig attempt_config = NULL;
        uint32_t attempt_visual = 0;

        // 选择优先格式：XRGB8888 > ARGB8888 > RGB565
        for (int j = 0; j < num_configs; j++) {
            EGLint native_visual = 0;
            eglGetConfigAttrib(ctx->egl_display, configs[j], EGL_NATIVE_VISUAL_ID, &native_visual);

            if ((uint32_t)native_visual == GBM_FORMAT_XRGB8888) {
                attempt_config = configs[j];
                attempt_visual = native_visual;
                break;
            }

            if (attempt_config == NULL && (uint32_t)native_visual == GBM_FORMAT_ARGB8888) {
                attempt_config = configs[j];
                attempt_visual = native_visual;
            }

            if (attempt_config == NULL && (uint32_t)native_visual == GBM_FORMAT_RGB565) {
                attempt_config = configs[j];
                attempt_visual = native_visual;
            }

            if (attempt_config == NULL) {
                attempt_config = configs[j];
                attempt_visual = native_visual;
            }
        }

        if (attempt_config) {
            ctx->config = attempt_config;
            fprintf(stderr, "DEBUG eglChooseConfig success at attempt %d (ES%d), num_configs=%d, native_visual=0x%x\n", i, try_versions[i], num_configs, attempt_visual);
            break;
        }
    }

    if (!ctx->config) {
        fprintf(stderr, "Failed to query EGL configs for ES3/ES2/minimal.\n");
        drm_egl_cleanup(ctx);
        return NULL;
    }

    // 创建EGL上下文，基于选择的版本
    EGLint context_version = use_es3 ? 3 : 2;
    EGLint ctx_attribs[] = {
        EGL_CONTEXT_CLIENT_VERSION, context_version,
        EGL_NONE
    };

    EGLint native_visual = 0;
    eglGetConfigAttrib(ctx->egl_display, ctx->config, EGL_NATIVE_VISUAL_ID, &native_visual);
    fprintf(stderr, "DEBUG config native visual = 0x%x (ES%d)\n", native_visual, context_version);

    ctx->egl_context = eglCreateContext(ctx->egl_display, ctx->config, EGL_NO_CONTEXT, ctx_attribs);
    if (ctx->egl_context == EGL_NO_CONTEXT) {
        fprintf(stderr, "Failed to create EGL context (version %d, error 0x%04x).\n", context_version, eglGetError());
        if (use_es3) {
            // ES3失败时尝试ES2
            context_version = 2;
            ctx_attribs[1] = 2;
            ctx->egl_context = eglCreateContext(ctx->egl_display, ctx->config, EGL_NO_CONTEXT, ctx_attribs);
        }
        if (ctx->egl_context == EGL_NO_CONTEXT) {
            drm_egl_cleanup(ctx);
            return NULL;
        }
    }

    // 创建GBM surface（按选中的EGL配置的 EGL_NATIVE_VISUAL_ID）
    uint32_t gbm_flags = GBM_BO_USE_RENDERING;
    if (!ctx->is_offscreen) {
        gbm_flags |= GBM_BO_USE_SCANOUT;
    }

    uint32_t gbm_format = GBM_FORMAT_XRGB8888;
    if (native_visual != 0) {
        gbm_format = (uint32_t)native_visual;
    }

    ctx->gbm_surf = gbm_surface_create(ctx->gbm_dev, surface_width, surface_height, gbm_format, gbm_flags);
    if (!ctx->gbm_surf) {
        fprintf(stderr, "Failed to create GBM surface with format 0x%x.\n", gbm_format);
        drm_egl_cleanup(ctx);
        return NULL;
    }

    // 创建EGL surface
    PFNEGLCREATEPLATFORMWINDOWSURFACEEXTPROC create_surface =
        (PFNEGLCREATEPLATFORMWINDOWSURFACEEXTPROC)eglGetProcAddress("eglCreatePlatformWindowSurfaceEXT");
    if (!create_surface) {
        fprintf(stderr, "Failed to get eglCreatePlatformWindowSurfaceEXT\n");
        drm_egl_cleanup(ctx);
        return NULL;
    }

    ctx->egl_surface = create_surface(ctx->egl_display, ctx->config, ctx->gbm_surf, NULL);
    if (ctx->egl_surface == EGL_NO_SURFACE) {
        EGLint err = eglGetError();
        fprintf(stderr, "Failed to create EGL surface (error 0x%04x). config native visual=0x%x\n", err, native_visual);
        drm_egl_cleanup(ctx);
        return NULL;
    }

    // 设置交换间隔并绑定上下文
    eglSwapInterval(ctx->egl_display, 1);
    if (!eglMakeCurrent(ctx->egl_display, ctx->egl_surface, ctx->egl_surface, ctx->egl_context)) {
        fprintf(stderr, "Failed to make EGL context current.\n");
        drm_egl_cleanup(ctx);
        return NULL;
    }
    return ctx;
}

void drm_egl_render_frame(DRMEGLContext* ctx) {
    // 交换EGL缓冲区
    eglSwapBuffers(ctx->egl_display, ctx->egl_surface);

    if (ctx->is_offscreen) {
        // 离屏渲染不需要页面翻转
        return;
    }

    // 获取下一帧 buffer
    ctx->next_bo = gbm_surface_lock_front_buffer(ctx->gbm_surf);
    if (!ctx->next_bo) {
        fprintf(stderr, "Failed to lock front buffer.\n");
        return;
    }

    uint32_t handle = gbm_bo_get_handle(ctx->next_bo).u32;
    uint32_t pitch = gbm_bo_get_stride(ctx->next_bo);

    // 创建 framebuffer
    if (drmModeAddFB(ctx->drm_fd, ctx->drm_mode.hdisplay, ctx->drm_mode.vdisplay, 24, 32, pitch, handle, &ctx->next_fb)) {
        perror("drmModeAddFB");
        return;
    }

    static bool first_frame = true;
    if (first_frame) {
        if (drmModeSetCrtc(ctx->drm_fd, ctx->saved_crtc->crtc_id, ctx->next_fb, 0, 0, &ctx->connector_id, 1, &ctx->drm_mode)) {
            perror("drmModeSetCrtc");
        }
        first_frame = false;
    } else {
        page_flipped = 0;
        // 使用页面翻转
        if (drmModePageFlip(ctx->drm_fd, ctx->saved_crtc->crtc_id, ctx->next_fb, DRM_MODE_PAGE_FLIP_EVENT, NULL)) {
            perror("drmModePageFlip");
            // 回退：直接设置CRTC（性能稍差）
            if (drmModeSetCrtc(ctx->drm_fd, ctx->saved_crtc->crtc_id, ctx->next_fb, 0, 0, &ctx->connector_id, 1, &ctx->drm_mode)) {
                perror("drmModeSetCrtc (fallback)");
            }
        } else {
            wait_for_page_flip(ctx->drm_fd);
        }

        // 释放上一帧 buffer
        if (ctx->current_bo) {
            drmModeRmFB(ctx->drm_fd, ctx->current_fb);
            gbm_surface_release_buffer(ctx->gbm_surf, ctx->current_bo);
        }
    }

    // 交换 buffer 指针
    ctx->current_bo = ctx->next_bo;
    ctx->current_fb = ctx->next_fb;
}

void drm_egl_cleanup(DRMEGLContext* ctx) {
    if (!ctx) return;
    
    if (ctx->egl_display != EGL_NO_DISPLAY) {
        eglMakeCurrent(ctx->egl_display, EGL_NO_SURFACE, EGL_NO_SURFACE, EGL_NO_CONTEXT);
        if (ctx->egl_context != EGL_NO_CONTEXT) {
            eglDestroyContext(ctx->egl_display, ctx->egl_context);
        }
        if (ctx->egl_surface != EGL_NO_SURFACE) {
            eglDestroySurface(ctx->egl_display, ctx->egl_surface);
        }
        eglTerminate(ctx->egl_display);
    }

    if (!ctx->is_offscreen) {
        if (ctx->current_bo) {
            drmModeRmFB(ctx->drm_fd, ctx->current_fb);
            gbm_surface_release_buffer(ctx->gbm_surf, ctx->current_bo);
        }

        if (ctx->next_bo) {
            drmModeRmFB(ctx->drm_fd, ctx->next_fb);
            gbm_surface_release_buffer(ctx->gbm_surf, ctx->next_bo);
        }

        if (ctx->saved_crtc) {
            drmModeSetCrtc(ctx->drm_fd, ctx->saved_crtc->crtc_id, ctx->saved_crtc->buffer_id,
                        ctx->saved_crtc->x, ctx->saved_crtc->y, &ctx->connector_id, 1, &ctx->saved_crtc->mode);
            drmModeFreeCrtc(ctx->saved_crtc);
        }
    }

    if (ctx->gbm_surf) {
        gbm_surface_destroy(ctx->gbm_surf);
    }

    if (ctx->gbm_dev) {
        gbm_device_destroy(ctx->gbm_dev);
    }

    if (ctx->drm_fd >= 0) {
        close(ctx->drm_fd);
    }

    free(ctx);
}