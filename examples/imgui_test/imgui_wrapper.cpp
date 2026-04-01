#include "imgui_wrapper.h"
#include "imgui/imgui.h"
#include "imgui/imgui_impl_opengl3.h"

#include "imgui/imgui.cpp"
#include "imgui/imgui_draw.cpp"
#include "imgui/imgui_tables.cpp"
#include "imgui/imgui_widgets.cpp"
#include "imgui/imgui_impl_opengl3.cpp"
extern "C" {

void go_imgui_platform_new_frame() {
    ImGuiIO& io = ImGui::GetIO();
    // Simulate no input for DRM environment
    io.MousePos = ImVec2(-FLT_MAX, -FLT_MAX);
    io.MouseDown[0] = false;
    io.MouseDown[1] = false;
    io.MouseDown[2] = false;
    io.MouseWheel = 0.0f;
    io.KeyCtrl = false;
    io.KeyShift = false;
    io.KeyAlt = false;
    io.KeySuper = false;
    for (int i = 0; i < IM_ARRAYSIZE(io.KeysDown); i++) io.KeysDown[i] = false;
}

void go_imgui_init(int width, int height) {
    IMGUI_CHECKVERSION();
    ImGui::CreateContext();
    ImGuiIO& io = ImGui::GetIO();
    io.DisplaySize = ImVec2((float)width, (float)height);
    io.DisplayFramebufferScale = ImVec2(1.0f, 1.0f);
    io.DeltaTime = 1.0f / 60.0f;
    io.ConfigFlags |= ImGuiConfigFlags_NavEnableKeyboard;
    ImGui::StyleColorsDark();
    ImGui_ImplOpenGL3_Init("#version 100");
}

void go_imgui_new_frame(int width, int height, float dt) {
    ImGuiIO& io = ImGui::GetIO();
    io.DisplaySize = ImVec2((float)width, (float)height);
    io.DeltaTime = dt;
    go_imgui_platform_new_frame();
    ImGui_ImplOpenGL3_NewFrame();
    ImGui::NewFrame();
}

void go_imgui_build_ui(int frame) {
    static float clear_color[4] = {0.14f, 0.16f, 0.19f, 1.0f};
    ImGui::Begin("g-drm-egl ImGui Test");
    ImGui::Text("Hello from imgui in DRM/EGL!");
    ImGui::Text("Frame: %d", frame);
    ImGui::Text("Press Ctrl+C to exit");
    ImGui::ColorEdit3("Clear Color", clear_color);
    ImGui::End();
}

void go_imgui_render() {
    ImGui::Render();
    ImGui_ImplOpenGL3_RenderDrawData(ImGui::GetDrawData());
}

void go_imgui_shutdown() {
    ImGui_ImplOpenGL3_Shutdown();
    ImGui::DestroyContext();
}

}
