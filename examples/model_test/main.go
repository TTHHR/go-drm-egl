package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/tthhr/go-drm-egl/drm"
)

const (
	vertexShaderSource = `#version 300 es
layout (location = 0) in vec3 aPos;
layout (location = 1) in vec3 aNormal;
layout (location = 2) in vec2 aTexCoord;

uniform mat4 uMVP;

out vec3 vNormal;
out vec2 vTexCoord;

void main() {
	gl_Position = uMVP * vec4(aPos, 1.0);
	vNormal = aNormal;
	vTexCoord = aTexCoord;
}`

	fragmentShaderSource = `#version 300 es
precision mediump float;

in vec3 vNormal;
in vec2 vTexCoord;

uniform sampler2D uTexture;
uniform int uUseTexture;
uniform vec4 uBaseColor;
uniform vec3 uLightDir;

out vec4 FragColor;

void main() {
	vec4 texColor = vec4(1.0);
	if (uUseTexture == 1) {
		texColor = texture(uTexture, vTexCoord);
	}

	vec4 base = texColor * uBaseColor;
	vec3 normal = normalize(vNormal);
	float light = max(dot(normal, normalize(-uLightDir)), 0.0);
	float ambient = 0.25;
	float diffuse = 0.75 * light;

	FragColor = vec4(base.rgb * (ambient + diffuse), base.a);
}`
)

const (
	glbMagic      = 0x46546C67
	glbVersion2   = 2
	chunkTypeJSON = 0x4E4F534A
	chunkTypeBIN  = 0x004E4942
)

type vec2 struct {
	x float32
	y float32
}

type vec3 struct {
	x float32
	y float32
	z float32
}

type vec4 struct {
	x float32
	y float32
	z float32
	w float32
}

type mat4 [16]float32

type gltfDocument struct {
	Scene       int              `json:"scene"`
	Scenes      []gltfScene      `json:"scenes"`
	Nodes       []gltfNode       `json:"nodes"`
	Meshes      []gltfMesh       `json:"meshes"`
	Materials   []gltfMaterial   `json:"materials"`
	Textures    []gltfTexture    `json:"textures"`
	Images      []gltfImage      `json:"images"`
	Accessors   []gltfAccessor   `json:"accessors"`
	BufferViews []gltfBufferView `json:"bufferViews"`
}

type gltfScene struct {
	Nodes []int `json:"nodes"`
}

type gltfNode struct {
	Mesh        int       `json:"mesh"`
	Children    []int     `json:"children"`
	Translation []float32 `json:"translation"`
	Rotation    []float32 `json:"rotation"`
	Scale       []float32 `json:"scale"`
	Matrix      []float32 `json:"matrix"`
}

type gltfMesh struct {
	Name       string          `json:"name"`
	Primitives []gltfPrimitive `json:"primitives"`
}

type gltfPrimitive struct {
	Attributes map[string]int `json:"attributes"`
	Indices    int            `json:"indices"`
	Material   int            `json:"material"`
}

type gltfMaterial struct {
	Name                 string `json:"name"`
	AlphaMode            string `json:"alphaMode"`
	PBRMetallicRoughness struct {
		BaseColorTexture *struct {
			Index int `json:"index"`
		} `json:"baseColorTexture"`
		BaseColorFactor []float32 `json:"baseColorFactor"`
	} `json:"pbrMetallicRoughness"`
}

type gltfTexture struct {
	Source int `json:"source"`
}

type gltfImage struct {
	BufferView int    `json:"bufferView"`
	MimeType   string `json:"mimeType"`
	Name       string `json:"name"`
}

type gltfAccessor struct {
	BufferView    int       `json:"bufferView"`
	ComponentType uint32    `json:"componentType"`
	Count         int       `json:"count"`
	Type          string    `json:"type"`
	ByteOffset    int       `json:"byteOffset"`
	Min           []float32 `json:"min"`
	Max           []float32 `json:"max"`
}

type gltfBufferView struct {
	Buffer     int `json:"buffer"`
	ByteOffset int `json:"byteOffset"`
	ByteLength int `json:"byteLength"`
	ByteStride int `json:"byteStride"`
}

type glbFile struct {
	doc *gltfDocument
	bin []byte
}

type materialGPU struct {
	textureID  uint32
	useTexture bool
	baseColor  vec4
}

type primitiveGPU struct {
	name      string
	vao       uint32
	vbo       uint32
	vertexCnt int32
	material  materialGPU
}

type bounds struct {
	min vec3
	max vec3
}

func main() {
	runtime.LockOSThread()

	modelPath, err := resolveModelPath("car.glb")
	if err != nil {
		panic(err)
	}

	glb, err := loadGLB(modelPath)
	if err != nil {
		panic(fmt.Sprintf("failed to load %s: %v", modelPath, err))
	}

	ctx, err := drm.Init(0, 0)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize DRM/EGL: %v", err))
	}
	defer ctx.Cleanup()

	if !drm.MakeCurrent(ctx) {
		panic("failed to make EGL context current")
	}
	drm.ViewPort(0, 0, ctx.Width(), ctx.Height())
	drm.Enable(drm.DEPTH_TEST)
	drm.Enable(drm.BLEND)
	drm.BlendFunc(drm.SRC_ALPHA, drm.ONE_MINUS_SRC_ALPHA)

	fmt.Printf("Initialized DRM/EGL: %dx%d\n", ctx.Width(), ctx.Height())
	fmt.Printf("Loading model: %s\n", modelPath)

	program := mustCreateProgram(vertexShaderSource, fragmentShaderSource)
	defer drm.DeleteProgram(program)

	uMVP := drm.GetUniformLocation(program, "uMVP")
	uTexture := drm.GetUniformLocation(program, "uTexture")
	uUseTexture := drm.GetUniformLocation(program, "uUseTexture")
	uBaseColor := drm.GetUniformLocation(program, "uBaseColor")
	uLightDir := drm.GetUniformLocation(program, "uLightDir")

	images, err := uploadImages(glb)
	if err != nil {
		panic(fmt.Sprintf("failed to upload textures: %v", err))
	}
	defer func() {
		for _, textureID := range images {
			if textureID != 0 {
				drm.DeleteTextures(textureID)
			}
		}
	}()

	primitives, sceneBounds, err := buildScene(glb, images)
	if err != nil {
		panic(fmt.Sprintf("failed to build model scene: %v", err))
	}
	defer func() {
		for _, primitive := range primitives {
			if primitive.vbo != 0 {
				drm.DeleteBuffers(primitive.vbo)
			}
			if primitive.vao != 0 {
				drm.DeleteVertexArrays(primitive.vao)
			}
		}
	}()

	fmt.Printf("Prepared %d drawable primitives\n", len(primitives))

	aspect := float32(ctx.Width()) / float32(ctx.Height())
	proj := perspective(45*math.Pi/180, aspect, 0.1, 200.0)
	center := sceneBounds.center()
	radius := sceneBounds.radius()
	if radius < 1 {
		radius = 1
	}
	lightDir := normalize3(vec3{0.4, 1.0, 0.3})

	start := time.Now()
	frameCount := 0
	lastFPS := time.Now()

	for {
		elapsed := time.Since(start).Seconds()
		cameraDistance := radius * 3.2
		cameraHeight := center.y + radius*0.9
		eye := vec3{
			x: center.x + float32(math.Cos(elapsed*0.45))*cameraDistance,
			y: cameraHeight,
			z: center.z + float32(math.Sin(elapsed*0.45))*cameraDistance,
		}
		view := lookAt(eye, center, vec3{0, 1, 0})
		mvp := mulMat4(proj, view)

		drm.ClearColor(0.06, 0.07, 0.10, 1.0)
		drm.Clear(drm.COLOR_BUFFER_BIT | drm.DEPTH_BUFFER_BIT)

		drm.UseProgram(program)
		drm.UniformMatrix4fv(uMVP, 1, false, &mvp[0])
		drm.Uniform3f(uLightDir, lightDir.x, lightDir.y, lightDir.z)

		for _, primitive := range primitives {
			drm.BindVertexArray(primitive.vao)
			drm.Uniform1i(uUseTexture, boolToInt32(primitive.material.useTexture))
			drm.Uniform4f(
				uBaseColor,
				primitive.material.baseColor.x,
				primitive.material.baseColor.y,
				primitive.material.baseColor.z,
				primitive.material.baseColor.w,
			)

			if primitive.material.useTexture {
				drm.ActiveTexture(drm.TEXTURE0)
				drm.BindTexture(drm.TEXTURE_2D, primitive.material.textureID)
				drm.Uniform1i(uTexture, 0)
			} else {
				drm.BindTexture(drm.TEXTURE_2D, 0)
			}

			drm.DrawArrays(drm.TRIANGLES, 0, primitive.vertexCnt)
		}

		drm.BindVertexArray(0)
		drm.BindTexture(drm.TEXTURE_2D, 0)
		drm.RenderFrame(ctx)

		frameCount++
		if time.Since(lastFPS) >= time.Second {
			fmt.Printf("FPS: %d\n", frameCount)
			frameCount = 0
			lastFPS = time.Now()
		}
	}
}

func resolveModelPath(name string) (string, error) {
	if _, err := os.Stat(name); err == nil {
		return name, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	path := filepath.Join(filepath.Dir(exe), name)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("model %q not found", name)
}

func loadGLB(path string) (*glbFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 20 {
		return nil, errors.New("glb file too small")
	}

	magic := binary.LittleEndian.Uint32(data[0:4])
	version := binary.LittleEndian.Uint32(data[4:8])
	length := binary.LittleEndian.Uint32(data[8:12])
	if magic != glbMagic {
		return nil, fmt.Errorf("invalid glb magic: 0x%x", magic)
	}
	if version != glbVersion2 {
		return nil, fmt.Errorf("unsupported glb version: %d", version)
	}
	if int(length) > len(data) {
		return nil, errors.New("glb length exceeds file size")
	}

	offset := 12
	var jsonChunk []byte
	var binChunk []byte
	for offset+8 <= int(length) {
		chunkLength := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		chunkType := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		offset += 8
		if offset+chunkLength > int(length) {
			return nil, errors.New("invalid glb chunk length")
		}
		chunk := data[offset : offset+chunkLength]
		offset += chunkLength

		switch chunkType {
		case chunkTypeJSON:
			jsonChunk = chunk
		case chunkTypeBIN:
			binChunk = chunk
		}
	}

	if len(jsonChunk) == 0 || len(binChunk) == 0 {
		return nil, errors.New("glb is missing JSON or BIN chunk")
	}

	var doc gltfDocument
	if err := json.Unmarshal(bytes.TrimRight(jsonChunk, "\x00 "), &doc); err != nil {
		return nil, err
	}

	return &glbFile{doc: &doc, bin: binChunk}, nil
}

func uploadImages(glb *glbFile) (map[int]uint32, error) {
	uploaded := make(map[int]uint32, len(glb.doc.Images))
	for i, img := range glb.doc.Images {
		viewData, err := glb.bufferViewData(img.BufferView)
		if err != nil {
			return nil, fmt.Errorf("image %d: %w", i, err)
		}
		decoded, _, err := image.Decode(bytes.NewReader(viewData))
		if err != nil {
			return nil, fmt.Errorf("decode image %d (%s): %w", i, img.Name, err)
		}

		rgba := toRGBA(decoded)
		texture := drm.GenTextures(1)
		drm.ActiveTexture(drm.TEXTURE0)
		drm.BindTexture(drm.TEXTURE_2D, texture)
		drm.TexParameteri(drm.TEXTURE_2D, drm.TEXTURE_WRAP_S, int32(drm.CLAMP_TO_EDGE))
		drm.TexParameteri(drm.TEXTURE_2D, drm.TEXTURE_WRAP_T, int32(drm.CLAMP_TO_EDGE))
		drm.TexParameteri(drm.TEXTURE_2D, drm.TEXTURE_MIN_FILTER, int32(drm.LINEAR))
		drm.TexParameteri(drm.TEXTURE_2D, drm.TEXTURE_MAG_FILTER, int32(drm.LINEAR))
		drm.TexImage2D(
			drm.TEXTURE_2D,
			0,
			drm.RGBA,
			int32(rgba.Rect.Dx()),
			int32(rgba.Rect.Dy()),
			0,
			drm.RGBA,
			drm.UNSIGNED_BYTE,
			drm.Ptr(rgba.Pix),
		)
		drm.BindTexture(drm.TEXTURE_2D, 0)

		uploaded[i] = texture
	}
	return uploaded, nil
}

func buildScene(glb *glbFile, images map[int]uint32) ([]primitiveGPU, bounds, error) {
	nodeWorlds := computeNodeWorlds(glb.doc)
	sceneIndex := glb.doc.Scene
	if sceneIndex < 0 || sceneIndex >= len(glb.doc.Scenes) {
		sceneIndex = 0
	}

	var (
		result      []primitiveGPU
		sceneBounds = newBounds()
	)

	scene := glb.doc.Scenes[sceneIndex]
	for _, nodeIndex := range scene.Nodes {
		if nodeIndex < 0 || nodeIndex >= len(glb.doc.Nodes) {
			continue
		}
		if err := buildNodePrimitives(glb, nodeIndex, nodeWorlds, images, &result, &sceneBounds); err != nil {
			return nil, bounds{}, err
		}
	}
	return result, sceneBounds, nil
}

func buildNodePrimitives(glb *glbFile, nodeIndex int, nodeWorlds []mat4, images map[int]uint32, result *[]primitiveGPU, sceneBounds *bounds) error {
	node := glb.doc.Nodes[nodeIndex]
	if node.Mesh >= 0 && node.Mesh < len(glb.doc.Meshes) {
		mesh := glb.doc.Meshes[node.Mesh]
		world := nodeWorlds[nodeIndex]
		for primitiveIndex, primitive := range mesh.Primitives {
			vertices, primitiveBounds, material, err := buildPrimitive(glb, primitive, world, images)
			if err != nil {
				return fmt.Errorf("mesh %q primitive %d: %w", mesh.Name, primitiveIndex, err)
			}
			if len(vertices) == 0 {
				continue
			}
			*sceneBounds = sceneBounds.merge(primitiveBounds)

			vao := drm.GenVertexArrays(1)
			vbo := drm.GenBuffers(1)
			drm.BindVertexArray(vao)
			drm.BindBuffer(drm.ARRAY_BUFFER, vbo)
			drm.BufferData(drm.ARRAY_BUFFER, vertices, drm.STATIC_DRAW)
			drm.VertexAttribPointer(0, 3, drm.FLOAT, false, 8*4, 0)
			drm.EnableVertexAttribArray(0)
			drm.VertexAttribPointer(1, 3, drm.FLOAT, false, 8*4, 3*4)
			drm.EnableVertexAttribArray(1)
			drm.VertexAttribPointer(2, 2, drm.FLOAT, false, 8*4, 6*4)
			drm.EnableVertexAttribArray(2)
			drm.BindBuffer(drm.ARRAY_BUFFER, 0)
			drm.BindVertexArray(0)

			*result = append(*result, primitiveGPU{
				name:      mesh.Name,
				vao:       vao,
				vbo:       vbo,
				vertexCnt: int32(len(vertices) / 8),
				material:  material,
			})
		}
	}

	for _, child := range node.Children {
		if child < 0 || child >= len(glb.doc.Nodes) {
			continue
		}
		if err := buildNodePrimitives(glb, child, nodeWorlds, images, result, sceneBounds); err != nil {
			return err
		}
	}
	return nil
}

func buildPrimitive(glb *glbFile, primitive gltfPrimitive, world mat4, images map[int]uint32) ([]float32, bounds, materialGPU, error) {
	posAccessor := primitive.Attributes["POSITION"]
	positions, err := glb.readFloatAccessor(posAccessor, 3)
	if err != nil {
		return nil, bounds{}, materialGPU{}, fmt.Errorf("read positions: %w", err)
	}
	vertexCount := len(positions) / 3
	if vertexCount == 0 {
		return nil, bounds{}, materialGPU{}, nil
	}

	var normals []float32
	if normalAccessor, ok := primitive.Attributes["NORMAL"]; ok {
		normals, err = glb.readFloatAccessor(normalAccessor, 3)
		if err != nil {
			return nil, bounds{}, materialGPU{}, fmt.Errorf("read normals: %w", err)
		}
	}

	var texcoords []float32
	if texAccessor, ok := primitive.Attributes["TEXCOORD_0"]; ok {
		texcoords, err = glb.readFloatAccessor(texAccessor, 2)
		if err != nil {
			return nil, bounds{}, materialGPU{}, fmt.Errorf("read texcoords: %w", err)
		}
	}

	indices, err := glb.readIndices(primitive.Indices)
	if err != nil {
		return nil, bounds{}, materialGPU{}, fmt.Errorf("read indices: %w", err)
	}
	if len(indices) == 0 {
		indices = make([]uint32, vertexCount)
		for i := 0; i < vertexCount; i++ {
			indices[i] = uint32(i)
		}
	}

	result := make([]float32, 0, len(indices)*8)
	primitiveBounds := newBounds()
	for _, idx := range indices {
		base := int(idx) * 3
		if base+2 >= len(positions) {
			return nil, bounds{}, materialGPU{}, fmt.Errorf("position index %d out of range", idx)
		}

		position := transformPoint(world, vec3{
			x: positions[base+0],
			y: positions[base+1],
			z: positions[base+2],
		})
		primitiveBounds = primitiveBounds.include(position)

		normal := vec3{0, 1, 0}
		if len(normals) >= base+3 {
			normal = normalize3(transformDirection(world, vec3{
				x: normals[base+0],
				y: normals[base+1],
				z: normals[base+2],
			}))
		}

		uv := vec2{}
		uvBase := int(idx) * 2
		if len(texcoords) >= uvBase+2 {
			uv = vec2{x: texcoords[uvBase+0], y: texcoords[uvBase+1]}
		}

		result = append(result,
			position.x, position.y, position.z,
			normal.x, normal.y, normal.z,
			uv.x, uv.y,
		)
	}

	return result, primitiveBounds, buildMaterial(glb.doc, primitive.Material, images), nil
}

func buildMaterial(doc *gltfDocument, materialIndex int, images map[int]uint32) materialGPU {
	material := materialGPU{
		baseColor: vec4{1, 1, 1, 1},
	}
	if materialIndex < 0 || materialIndex >= len(doc.Materials) {
		return material
	}

	src := doc.Materials[materialIndex]
	if len(src.PBRMetallicRoughness.BaseColorFactor) == 4 {
		material.baseColor = vec4{
			x: src.PBRMetallicRoughness.BaseColorFactor[0],
			y: src.PBRMetallicRoughness.BaseColorFactor[1],
			z: src.PBRMetallicRoughness.BaseColorFactor[2],
			w: src.PBRMetallicRoughness.BaseColorFactor[3],
		}
	}

	if src.PBRMetallicRoughness.BaseColorTexture != nil {
		textureIndex := src.PBRMetallicRoughness.BaseColorTexture.Index
		if textureIndex >= 0 && textureIndex < len(doc.Textures) {
			imageIndex := doc.Textures[textureIndex].Source
			textureID := images[imageIndex]
			if textureID != 0 {
				material.textureID = textureID
				material.useTexture = true
			}
		}
	}

	return material
}

func computeNodeWorlds(doc *gltfDocument) []mat4 {
	worlds := make([]mat4, len(doc.Nodes))
	for i := range worlds {
		worlds[i] = identity()
	}

	sceneIndex := doc.Scene
	if sceneIndex < 0 || sceneIndex >= len(doc.Scenes) {
		sceneIndex = 0
	}
	if sceneIndex >= len(doc.Scenes) {
		return worlds
	}

	for _, root := range doc.Scenes[sceneIndex].Nodes {
		if root >= 0 && root < len(doc.Nodes) {
			computeNodeWorldRecursive(doc, root, identity(), worlds)
		}
	}
	return worlds
}

func computeNodeWorldRecursive(doc *gltfDocument, nodeIndex int, parent mat4, worlds []mat4) {
	local := nodeLocalMatrix(doc.Nodes[nodeIndex])
	world := mulMat4(parent, local)
	worlds[nodeIndex] = world
	for _, child := range doc.Nodes[nodeIndex].Children {
		if child >= 0 && child < len(doc.Nodes) {
			computeNodeWorldRecursive(doc, child, world, worlds)
		}
	}
}

func nodeLocalMatrix(node gltfNode) mat4 {
	if len(node.Matrix) == 16 {
		var m mat4
		copy(m[:], node.Matrix)
		return m
	}

	translation := vec3{}
	if len(node.Translation) == 3 {
		translation = vec3{node.Translation[0], node.Translation[1], node.Translation[2]}
	}

	rotation := vec4{0, 0, 0, 1}
	if len(node.Rotation) == 4 {
		rotation = vec4{node.Rotation[0], node.Rotation[1], node.Rotation[2], node.Rotation[3]}
	}

	scale := vec3{1, 1, 1}
	if len(node.Scale) == 3 {
		scale = vec3{node.Scale[0], node.Scale[1], node.Scale[2]}
	}

	return mulMat4(translationMatrix(translation), mulMat4(quatMatrix(rotation), scaleMatrix(scale)))
}

func mustCreateProgram(vertexSource, fragmentSource string) uint32 {
	vertexShader := drm.CreateShader(drm.VERTEX_SHADER)
	drm.ShaderSource(vertexShader, vertexSource)
	drm.CompileShader(vertexShader)
	checkShader(vertexShader, "vertex")

	fragmentShader := drm.CreateShader(drm.FRAGMENT_SHADER)
	drm.ShaderSource(fragmentShader, fragmentSource)
	drm.CompileShader(fragmentShader)
	checkShader(fragmentShader, "fragment")

	program := drm.CreateProgram()
	drm.AttachShader(program, vertexShader)
	drm.AttachShader(program, fragmentShader)
	drm.LinkProgram(program)
	checkProgram(program)

	drm.DeleteShader(vertexShader)
	drm.DeleteShader(fragmentShader)
	return program
}

func checkShader(shader uint32, stage string) {
	var status int32
	drm.GetShaderiv(shader, drm.COMPILE_STATUS, &status)
	if status != 0 {
		return
	}

	var logLength int32
	drm.GetShaderiv(shader, drm.INFO_LOG_LENGTH, &logLength)
	logData := make([]byte, logLength)
	drm.GetShaderInfoLog(shader, logLength, nil, &logData[0])
	panic(fmt.Sprintf("%s shader compile failed: %s", stage, string(bytes.TrimRight(logData, "\x00"))))
}

func checkProgram(program uint32) {
	var status int32
	drm.GetProgramiv(program, drm.LINK_STATUS, &status)
	if status != 0 {
		return
	}

	var logLength int32
	drm.GetProgramiv(program, drm.INFO_LOG_LENGTH, &logLength)
	logData := make([]byte, logLength)
	drm.GetProgramInfoLog(program, logLength, nil, &logData[0])
	panic(fmt.Sprintf("program link failed: %s", string(bytes.TrimRight(logData, "\x00"))))
}

func (g *glbFile) bufferViewData(index int) ([]byte, error) {
	if index < 0 || index >= len(g.doc.BufferViews) {
		return nil, fmt.Errorf("bufferView %d out of range", index)
	}
	view := g.doc.BufferViews[index]
	start := view.ByteOffset
	end := start + view.ByteLength
	if start < 0 || end > len(g.bin) || start > end {
		return nil, fmt.Errorf("bufferView %d has invalid range", index)
	}
	return g.bin[start:end], nil
}

func (g *glbFile) readFloatAccessor(index int, width int) ([]float32, error) {
	if index < 0 || index >= len(g.doc.Accessors) {
		return nil, fmt.Errorf("accessor %d out of range", index)
	}
	accessor := g.doc.Accessors[index]
	if accessor.ComponentType != 5126 {
		return nil, fmt.Errorf("accessor %d expected float32, got %d", index, accessor.ComponentType)
	}
	if typeWidth(accessor.Type) != width {
		return nil, fmt.Errorf("accessor %d expected width %d, got %s", index, width, accessor.Type)
	}

	viewData, err := g.bufferViewData(accessor.BufferView)
	if err != nil {
		return nil, err
	}
	stride := g.doc.BufferViews[accessor.BufferView].ByteStride
	if stride == 0 {
		stride = width * 4
	}
	offset := accessor.ByteOffset
	result := make([]float32, accessor.Count*width)
	for i := 0; i < accessor.Count; i++ {
		base := offset + i*stride
		for j := 0; j < width; j++ {
			begin := base + j*4
			if begin+4 > len(viewData) {
				return nil, fmt.Errorf("accessor %d overflows bufferView", index)
			}
			result[i*width+j] = math.Float32frombits(binary.LittleEndian.Uint32(viewData[begin : begin+4]))
		}
	}
	return result, nil
}

func (g *glbFile) readIndices(index int) ([]uint32, error) {
	if index < 0 {
		return nil, nil
	}
	if index >= len(g.doc.Accessors) {
		return nil, fmt.Errorf("accessor %d out of range", index)
	}
	accessor := g.doc.Accessors[index]
	if accessor.Type != "SCALAR" {
		return nil, fmt.Errorf("index accessor %d is not SCALAR", index)
	}
	viewData, err := g.bufferViewData(accessor.BufferView)
	if err != nil {
		return nil, err
	}

	componentSize := componentSize(accessor.ComponentType)
	if componentSize == 0 {
		return nil, fmt.Errorf("unsupported index component type %d", accessor.ComponentType)
	}
	stride := g.doc.BufferViews[accessor.BufferView].ByteStride
	if stride == 0 {
		stride = componentSize
	}

	result := make([]uint32, accessor.Count)
	for i := 0; i < accessor.Count; i++ {
		base := accessor.ByteOffset + i*stride
		if base+componentSize > len(viewData) {
			return nil, fmt.Errorf("index accessor %d overflows bufferView", index)
		}
		switch accessor.ComponentType {
		case 5121:
			result[i] = uint32(viewData[base])
		case 5123:
			result[i] = uint32(binary.LittleEndian.Uint16(viewData[base : base+2]))
		case 5125:
			result[i] = binary.LittleEndian.Uint32(viewData[base : base+4])
		default:
			return nil, fmt.Errorf("unsupported index component type %d", accessor.ComponentType)
		}
	}
	return result, nil
}

func componentSize(componentType uint32) int {
	switch componentType {
	case 5121:
		return 1
	case 5123:
		return 2
	case 5125, 5126:
		return 4
	default:
		return 0
	}
}

func typeWidth(kind string) int {
	switch kind {
	case "SCALAR":
		return 1
	case "VEC2":
		return 2
	case "VEC3":
		return 3
	case "VEC4":
		return 4
	default:
		return 0
	}
}

func boolToInt32(v bool) int32 {
	if v {
		return 1
	}
	return 0
}

func toRGBA(img image.Image) *image.RGBA {
	b := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			rgba.Set(x-b.Min.X, y-b.Min.Y, color.RGBAModel.Convert(img.At(x, y)))
		}
	}
	return rgba
}

func identity() mat4 {
	return mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
}

func translationMatrix(v vec3) mat4 {
	m := identity()
	m[12] = v.x
	m[13] = v.y
	m[14] = v.z
	return m
}

func scaleMatrix(v vec3) mat4 {
	return mat4{
		v.x, 0, 0, 0,
		0, v.y, 0, 0,
		0, 0, v.z, 0,
		0, 0, 0, 1,
	}
}

func quatMatrix(q vec4) mat4 {
	x := q.x
	y := q.y
	z := q.z
	w := q.w

	xx := x * x
	yy := y * y
	zz := z * z
	xy := x * y
	xz := x * z
	yz := y * z
	wx := w * x
	wy := w * y
	wz := w * z

	return mat4{
		1 - 2*(yy+zz), 2 * (xy + wz), 2 * (xz - wy), 0,
		2 * (xy - wz), 1 - 2*(xx+zz), 2 * (yz + wx), 0,
		2 * (xz + wy), 2 * (yz - wx), 1 - 2*(xx+yy), 0,
		0, 0, 0, 1,
	}
}

func mulMat4(a, b mat4) mat4 {
	var r mat4
	for col := 0; col < 4; col++ {
		for row := 0; row < 4; row++ {
			r[col*4+row] =
				a[0*4+row]*b[col*4+0] +
					a[1*4+row]*b[col*4+1] +
					a[2*4+row]*b[col*4+2] +
					a[3*4+row]*b[col*4+3]
		}
	}
	return r
}

func transformPoint(m mat4, v vec3) vec3 {
	return vec3{
		x: m[0]*v.x + m[4]*v.y + m[8]*v.z + m[12],
		y: m[1]*v.x + m[5]*v.y + m[9]*v.z + m[13],
		z: m[2]*v.x + m[6]*v.y + m[10]*v.z + m[14],
	}
}

func transformDirection(m mat4, v vec3) vec3 {
	return vec3{
		x: m[0]*v.x + m[4]*v.y + m[8]*v.z,
		y: m[1]*v.x + m[5]*v.y + m[9]*v.z,
		z: m[2]*v.x + m[6]*v.y + m[10]*v.z,
	}
}

func perspective(fovy, aspect, near, far float32) mat4 {
	f := float32(1.0 / math.Tan(float64(fovy/2)))
	return mat4{
		f / aspect, 0, 0, 0,
		0, f, 0, 0,
		0, 0, (far + near) / (near - far), -1,
		0, 0, (2 * far * near) / (near - far), 0,
	}
}

func lookAt(eye, center, up vec3) mat4 {
	f := normalize3(sub3(center, eye))
	s := normalize3(cross(f, up))
	u := cross(s, f)

	return mat4{
		s.x, u.x, -f.x, 0,
		s.y, u.y, -f.y, 0,
		s.z, u.z, -f.z, 0,
		-dot3(s, eye), -dot3(u, eye), dot3(f, eye), 1,
	}
}

func add3(a, b vec3) vec3 {
	return vec3{a.x + b.x, a.y + b.y, a.z + b.z}
}

func sub3(a, b vec3) vec3 {
	return vec3{a.x - b.x, a.y - b.y, a.z - b.z}
}

func scale3(v vec3, s float32) vec3 {
	return vec3{v.x * s, v.y * s, v.z * s}
}

func dot3(a, b vec3) float32 {
	return a.x*b.x + a.y*b.y + a.z*b.z
}

func cross(a, b vec3) vec3 {
	return vec3{
		x: a.y*b.z - a.z*b.y,
		y: a.z*b.x - a.x*b.z,
		z: a.x*b.y - a.y*b.x,
	}
}

func length3(v vec3) float32 {
	return float32(math.Sqrt(float64(dot3(v, v))))
}

func normalize3(v vec3) vec3 {
	l := length3(v)
	if l == 0 {
		return vec3{0, 1, 0}
	}
	return scale3(v, 1/l)
}

func newBounds() bounds {
	large := float32(1 << 30)
	return bounds{
		min: vec3{large, large, large},
		max: vec3{-large, -large, -large},
	}
}

func (b bounds) include(v vec3) bounds {
	if v.x < b.min.x {
		b.min.x = v.x
	}
	if v.y < b.min.y {
		b.min.y = v.y
	}
	if v.z < b.min.z {
		b.min.z = v.z
	}
	if v.x > b.max.x {
		b.max.x = v.x
	}
	if v.y > b.max.y {
		b.max.y = v.y
	}
	if v.z > b.max.z {
		b.max.z = v.z
	}
	return b
}

func (b bounds) merge(other bounds) bounds {
	b = b.include(other.min)
	b = b.include(other.max)
	return b
}

func (b bounds) center() vec3 {
	return scale3(add3(b.min, b.max), 0.5)
}

func (b bounds) radius() float32 {
	return length3(sub3(b.max, b.min)) * 0.5
}
