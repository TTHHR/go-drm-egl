#define IMGUI_IMPL_OPENGL_ES2
#define IMGUI_DEFINE_MATH_OPERATORS

#ifdef __cplusplus
extern "C" {
#endif

void go_imgui_platform_new_frame();

void go_imgui_init(int width, int height);

void go_imgui_new_frame(int width, int height, float dt);
void go_imgui_build_ui(int frame) ;

void go_imgui_render();

void go_imgui_shutdown();
#ifdef __cplusplus
}
#endif
