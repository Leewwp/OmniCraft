import assert from "node:assert/strict";
import test from "node:test";
import { JSDOM } from "jsdom";

/**
 * #689 3D 查看器 seam：三视图坐标合同、预算估算纯函数，以及
 * stl/obj/ply/3mf/gcode 五格式 loader 真实解析验证（three loader 为纯
 * JS 解析、无 WebGL 依赖，可在 node 内验证；WebGL 渲染面归 #691 真机
 * 冒烟）。测试样本程序化生成（等价于入库样本，避免二进制 fixture）。
 */

import { VIEW_CONTRACT, estimateBinaryStlTriangles, fileExtensionOf } from "@/components/content/ModelViewer";

// ThreeMFLoader 经 DOMParser 解析模型 XML：在 loader 动态加载前注入
// jsdom globals（模块体先于任何 test 体执行）。
const dom = new JSDOM("<!doctype html><html><body></body></html>", { url: "http://localhost/" });
(globalThis as Record<string, unknown>).DOMParser = dom.window.DOMParser;
(globalThis as Record<string, unknown>).window = dom.window;

test("view contract: Z-up build plate — Top=+Z / Front=-Y / Side=+X, orthographic", () => {
  assert.deepEqual(VIEW_CONTRACT.top.dir, [0, 0, 1]);
  assert.deepEqual(VIEW_CONTRACT.front.dir, [0, -1, 0]);
  assert.deepEqual(VIEW_CONTRACT.side.dir, [1, 0, 0]);
  for (const spec of Object.values(VIEW_CONTRACT)) assert.equal(spec.orthographic, true);
});

test("binary STL triangle estimation: 84-byte header + 50 bytes per facet", () => {
  assert.equal(estimateBinaryStlTriangles(84), 0);
  assert.equal(estimateBinaryStlTriangles(84 + 50), 1);
  assert.equal(estimateBinaryStlTriangles(84 + 50 * 1_000_000), 1_000_000);
});

test("fileExtensionOf lowercases with dot", () => {
  assert.equal(fileExtensionOf("Benchy.STL"), ".stl");
  assert.equal(fileExtensionOf("part.gcode"), ".gcode");
  assert.equal(fileExtensionOf("noext"), "");
});

/** 生成单面二进制 STL（84B 头 + 1 面 × 50B）。 */
function binaryStlOneFacet(): ArrayBuffer {
  const buffer = new ArrayBuffer(84 + 50);
  const view = new DataView(buffer);
  view.setUint32(80, 1, true); // facet count
  const facet = new Float32Array(buffer, 84, 12);
  const [n, a, b, c] = [
    [0, 0, 1],
    [0, 0, 0],
    [1, 0, 0],
    [0, 1, 0],
  ];
  facet.set([...n, ...a, ...b, ...c], 0);
  return buffer;
}

test("STL loader parses the generated single-facet binary sample", async () => {
  const { STLLoader } = await import("three/examples/jsm/loaders/STLLoader.js");
  const geometry = new STLLoader().parse(binaryStlOneFacet());
  assert.equal(geometry.attributes.position.count, 3, "one facet = three vertices");
});

test("OBJ loader parses a cube sample as a plain mesh (no material matching, V1)", async () => {
  const { OBJLoader } = await import("three/examples/jsm/loaders/OBJLoader.js");
  const objText = [
    "v 0 0 0", "v 1 0 0", "v 1 1 0", "v 0 1 0",
    "v 0 0 1", "v 1 0 1", "v 1 1 1", "v 0 1 1",
    "f 1 2 3 4", "f 5 6 7 8",
  ].join("\n");
  const object = new OBJLoader().parse(objText);
  let vertices = 0;
  object.traverse((raw: object) => {
    const child = raw as { isMesh?: boolean; geometry?: { attributes: Record<string, { count: number }> } };
    if (child.isMesh && child.geometry) vertices += child.geometry.attributes.position?.count ?? 0;
  });
  // OBJLoader 将四边形面三角化：2 个 quad → 4 三角 → 12 顶点位。
  assert.equal(vertices, 12, "two quad faces triangulate to 12 vertex slots");
});

test("PLY loader parses an ascii PLY sample", async () => {
  const { PLYLoader } = await import("three/examples/jsm/loaders/PLYLoader.js");
  const plyText = [
    "ply", "format ascii 1.0",
    "element vertex 3",
    "property float x", "property float y", "property float z",
    "element face 1",
    "property list uchar int vertex_indices",
    "end_header",
    "0 0 0", "1 0 0", "0 1 0",
    "3 0 1 2",
  ].join("\n");
  const geometry = new PLYLoader().parse(plyText);
  assert.equal(geometry.attributes.position.count, 3);
});

test("GCode loader parses a two-segment sample (synchronous parse semantics)", async () => {
  const { GCodeLoader } = await import("three/examples/jsm/loaders/GCodeLoader.js");
  const gcode = ["G28", "G1 X0 Y0 Z0.2 E0", "G1 X10 Y0 Z0.2 E1", "G1 X10 Y10 Z0.2 E2"].join("\n");
  const object = new GCodeLoader().parse(gcode);
  assert.ok(object, "gcode parse returns an object");
});

test("3MF loader parses a minimal 3D/*.model package", async () => {
  const { ThreeMFLoader } = await import("three/examples/jsm/loaders/3MFLoader.js");
  // 程序化构造最小 3MF（zip 容器）：[Content_Types].xml + 3D/3dmodel.model。
  const modelXml = `<?xml version="1.0" encoding="UTF-8"?>
<model unit="millimeter" xml:lang="en-US" xmlns="http://schemas.microsoft.com/3dmanufacturing/core/2015/02">
  <resources><object id="1" type="model"><mesh><vertices>
    <vertex x="0" y="0" z="0"/><vertex x="1" y="0" z="0"/><vertex x="0" y="1" z="0"/>
  </vertices><triangles><triangle v1="0" v2="1" v3="2"/></triangles></mesh></object></resources>
  <build><item objectid="1"/></build>
</model>`;
  const contentTypes = `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="model" ContentType="application/vnd.ms-package.3dmanufacturing-3dmodel+xml"/>
</Types>`;
  const rels = `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Target="/3D/3dmodel.model" Id="rel0" Type="http://schemas.microsoft.com/3dmanufacturing/2013/01/3dmodel"/>
</Relationships>`;
  const buffer = await buildZip({
    "[Content_Types].xml": contentTypes,
    "_rels/.rels": rels,
    "3D/3dmodel.model": modelXml,
  });
  const object = new ThreeMFLoader().parse(buffer);
  let triangles = 0;
  object.traverse((raw: object) => {
    const child = raw as { isMesh?: boolean; geometry?: { index: { count: number } | null; attributes: Record<string, { count: number }> } };
    if (child.isMesh && child.geometry) {
      triangles += child.geometry.index ? child.geometry.index.count / 3 : (child.geometry.attributes.position?.count ?? 0) / 3;
    }
  });
  assert.equal(triangles, 1, "the single triangle from the model part is loaded");
});

test("dispatcher contract: model3d routes by extension, .mtl falls back to download card", () => {
  const dispatch = (att: { file_type?: string; oss_url?: string; scan_status?: string; original_file_name?: string }): "scan-card" | "viewer" | "download-card" => {
    if (att.scan_status && att.scan_status !== "clean" && att.scan_status !== "not_required") return "scan-card";
    if (!att.oss_url) return "download-card";
    if (att.file_type === "document") return "viewer";
    if (att.file_type === "model3d" && !(att.original_file_name ?? "").toLowerCase().endsWith(".mtl")) return "viewer";
    return "download-card";
  };
  assert.equal(dispatch({ file_type: "model3d", oss_url: "u", original_file_name: "part.stl" }), "viewer");
  assert.equal(dispatch({ file_type: "model3d", oss_url: "u", original_file_name: "part.3mf" }), "viewer");
  assert.equal(dispatch({ file_type: "model3d", oss_url: "u", original_file_name: "part.mtl" }), "download-card", "V1: MTL never previews");
  assert.equal(dispatch({ file_type: "model3d", scan_status: "pending" }), "scan-card", "non-clean model3d has no preview");
});

/** 纯 node zip 构造：fflate（three 3MF loader 同源依赖，必然在依赖树内）。 */
async function buildZip(entries: Record<string, string>): Promise<ArrayBuffer> {
  const { zipSync } = await import("fflate");
  const files: Record<string, Uint8Array> = {};
  for (const [name, content] of Object.entries(entries)) {
    files[name] = new TextEncoder().encode(content);
  }
  const zipped = zipSync(files);
  return zipped.buffer.slice(zipped.byteOffset, zipped.byteOffset + zipped.byteLength) as ArrayBuffer;
}
