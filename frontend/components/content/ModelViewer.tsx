"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { AlertTriangle, Box, FileText, Loader2 } from "lucide-react";

import { DownloadButton } from "@/components/content/DownloadButton";

/**
 * ModelViewer（#689）：three.js 精确锁版 + OrbitControls（阻尼拖转/滚轮
 * 缩放）+ 自动居中 fit-to-view + XY 底面网格 + 三视图按钮。
 *
 * 坐标合同（spec v2）：统一 Z-up、XY=build plate——Top=+Z / Front=-Y /
 * Side=+X；三视图按钮用正交机位，Orbit 交互保持透视。
 *
 * 预算合同（v2）：上传上限（50MB）≠ 预览能力——model3d_max_preview_mb /
 * model3d_max_triangles / gcode_max_lines 独立配置；超限优雅降级「文件
 * 较大，暂不提供在线预览」+ 下载入口，正常发布与下载不受影响。
 * GCode 解析为同步语义（three GCodeLoader），不假设非阻塞——按行数
 * 预算先行拒绝，不做全量解析。
 *
 * V1 无 OBJ+MTL 材质匹配（OSS key 随机化使 basename 匹配不可实现）：
 * OBJ 素模预览；MTL 由分发层落下载卡，不进入本组件。
 */

const DEFAULT_MAX_PREVIEW_MB = 15;
const DEFAULT_MAX_TRIANGLES = 1_000_000;
const DEFAULT_GCODE_MAX_LINES = 500_000;

type ViewerState =
  | { kind: "loading"; progress: number }
  | { kind: "degraded"; reason: "size" | "triangles" | "gcodeLines" }
  | { kind: "error" }
  | { kind: "ready" };

export type ViewName = "orbit" | "front" | "side" | "top";

/** 三视图坐标合同（导出供单测）：Z-up 下各视图的单位观察方向与 up。 */
export const VIEW_CONTRACT: Record<Exclude<ViewName, "orbit">, { dir: [number, number, number]; orthographic: true }> = {
  // Front = -Y（从 Y 负方向看 XY 板面），Side = +X，Top = +Z。
  front: { dir: [0, -1, 0], orthographic: true },
  side: { dir: [1, 0, 0], orthographic: true },
  top: { dir: [0, 0, 1], orthographic: true },
};

/** 估算二进制 STL 三角面数（导出供单测）：84 字节头 + 50 字节/面。 */
export function estimateBinaryStlTriangles(byteSize: number): number {
  if (byteSize <= 84) return 0;
  return Math.floor((byteSize - 84) / 50);
}

export function fileExtensionOf(name: string): string {
  const dot = name.lastIndexOf(".");
  return dot >= 0 ? name.slice(dot).toLowerCase() : "";
}

export interface ModelViewerProps {
  url: string;
  fileName: string;
  fileSize?: number;
  maxPreviewMB?: number;
  maxTriangles?: number;
  gcodeMaxLines?: number;
  contentId: number;
  attachmentId: number;
  allowCopy: boolean;
}

export function ModelViewer({
  url,
  fileName,
  fileSize,
  maxPreviewMB,
  maxTriangles,
  gcodeMaxLines,
  contentId,
  attachmentId,
  allowCopy,
}: ModelViewerProps) {
  const t = useTranslations();
  const [state, setState] = useState<ViewerState>({ kind: "loading", progress: 0 });
  const [view, setView] = useState<ViewName>("orbit");
  const mountRef = useRef<HTMLDivElement | null>(null);
  const viewControllerRef = useRef<{ apply: (view: ViewName) => void } | null>(null);
  const ext = useMemo(() => fileExtensionOf(fileName), [fileName]);
  const budgetMB = maxPreviewMB && maxPreviewMB > 0 ? maxPreviewMB : DEFAULT_MAX_PREVIEW_MB;
  const triangleBudget = maxTriangles && maxTriangles > 0 ? maxTriangles : DEFAULT_MAX_TRIANGLES;
  const gcodeBudget = gcodeMaxLines && gcodeMaxLines > 0 ? gcodeMaxLines : DEFAULT_GCODE_MAX_LINES;

  useEffect(() => {
    let dispose: (() => void) | undefined;
    let cancelled = false;
    setState({ kind: "loading", progress: 5 });

    if (fileSize != null && fileSize > budgetMB * 1024 * 1024) {
      setState({ kind: "degraded", reason: "size" });
      return;
    }

    (async () => {
      try {
        const res = await fetch(url);
        if (!res.ok) throw new Error(`fetch failed: ${res.status}`);
        setState({ kind: "loading", progress: 30 });
        const buffer = await res.arrayBuffer();
        if (cancelled) return;
        setState({ kind: "loading", progress: 55 });

        // 预算先行：解析前估计规模，超限直接降级（不做全量解析）。
        if (ext === ".stl" && estimateBinaryStlTriangles(buffer.byteLength) > triangleBudget) {
          setState({ kind: "degraded", reason: "triangles" });
          return;
        }
        if (ext === ".gcode") {
          const text = new TextDecoder().decode(buffer);
          const lines = text.split("\n").length;
          if (lines > gcodeBudget) {
            setState({ kind: "degraded", reason: "gcodeLines" });
            return;
          }
        }

        // three 只在客户端懒分块内加载（本组件本身经 next/dynamic 引入）。
        const THREE = await import("three");
        const { OrbitControls } = await import("three/examples/jsm/controls/OrbitControls.js");

        let object: InstanceType<typeof THREE.Object3D>;
        if (ext === ".stl") {
          const { STLLoader } = await import("three/examples/jsm/loaders/STLLoader.js");
          const geometry = new STLLoader().parse(buffer);
          applyBudgetToGeometry(geometry, triangleBudget);
          object = new THREE.Mesh(geometry, defaultMaterial(THREE));
        } else if (ext === ".obj") {
          const { OBJLoader } = await import("three/examples/jsm/loaders/OBJLoader.js");
          object = new OBJLoader().parse(new TextDecoder().decode(buffer));
          countGroupTriangles(object, triangleBudget);
        } else if (ext === ".ply") {
          const { PLYLoader } = await import("three/examples/jsm/loaders/PLYLoader.js");
          const geometry = new PLYLoader().parse(buffer);
          applyBudgetToGeometry(geometry, triangleBudget);
          object = new THREE.Mesh(geometry, defaultMaterial(THREE));
        } else if (ext === ".3mf") {
          const { ThreeMFLoader } = await import("three/examples/jsm/loaders/3MFLoader.js");
          const loaded = new ThreeMFLoader().parse(buffer);
          countGroupTriangles(loaded, triangleBudget);
          object = loaded;
        } else if (ext === ".gcode") {
          const { GCodeLoader } = await import("three/examples/jsm/loaders/GCodeLoader.js");
          const loaded = new GCodeLoader().parse(new TextDecoder().decode(buffer));
          countGroupTriangles(loaded, triangleBudget);
          object = loaded;
        } else {
          setState({ kind: "error" });
          return;
        }
        if (cancelled) return;
        setState({ kind: "loading", progress: 80 });

        const controller = mountScene(THREE, OrbitControls, object, mountRef.current, () => setView("orbit"), view);
        viewControllerRef.current = controller;
        dispose = controller;
        if (cancelled) {
          dispose?.();
          return;
        }
        setState({ kind: "ready" });
      } catch {
        if (!cancelled) setState({ kind: "error" });
      }
    })();

    return () => {
      cancelled = true;
      dispose?.();
    };
    // view intentionally excluded: the view switch repositions the active
    // camera via the in-scene controller below rather than re-parsing.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [url, ext, fileSize, budgetMB, triangleBudget, gcodeBudget]);

  const downloadEntry = allowCopy ? (
    <DownloadButton contentId={contentId} attachmentId={attachmentId} contentType="model3d" size="sm" />
  ) : null;

  /* 场景容器跨状态常驻（hidden 而非卸载）：mountScene 在异步加载完成时向
     ref 容器追加 canvas——loading 态若卸载容器，ref 为 null、canvas 永不
     出现（#691 真机冒烟实证，jsdom 测试只覆盖解析不覆盖挂载）。 */
  const testid =
    state.kind === "ready" ? "model-viewer-ready"
    : state.kind === "loading" ? "model-viewer-loading"
    : state.kind === "degraded" ? "model-viewer-degraded"
    : "model-viewer-error";

  return (
    <div className="space-y-2" data-testid={testid}>
      <div
        ref={mountRef}
        className={state.kind === "ready" ? "h-80 w-full overflow-hidden rounded-lg border border-border bg-card" : "hidden"}
        role="img"
        aria-label={t("content.attachmentPreview.model.canvasLabel", { name: fileName })}
      />
      {state.kind === "loading" && (
        <div className="flex items-center gap-2 rounded-lg border border-border bg-card p-4 text-sm text-muted-foreground" role="status">
          <Loader2 className="h-4 w-4 animate-spin" />
          {t("content.attachmentPreview.model.loading", { progress: state.progress })}
        </div>
      )}
      {state.kind === "degraded" && (
        <div className="flex items-center justify-between gap-3 rounded-lg border border-border bg-card p-4">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <FileText className="h-4 w-4 shrink-0" />
            {t("content.attachmentPreview.tooLarge")}
          </div>
          {downloadEntry}
        </div>
      )}
      {state.kind === "error" && (
        <div className="flex items-center justify-between gap-3 rounded-lg border border-border bg-card p-4">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <AlertTriangle className="h-4 w-4 shrink-0 text-destructive" />
            {t("content.attachmentPreview.failed")}
          </div>
          {downloadEntry}
        </div>
      )}
      {state.kind === "ready" && (
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex gap-1" role="group" aria-label={t("content.attachmentPreview.model.views")}>
          {(["orbit", "front", "side", "top"] as ViewName[]).map((name) => (
            <button
              key={name}
              type="button"
              aria-pressed={view === name}
              className={`rounded-md px-2.5 py-1 text-xs ${view === name ? "bg-primary text-primary-foreground" : "bg-muted text-muted-foreground"}`}
              onClick={() => {
                setView(name);
                viewControllerRef.current?.apply(name);
              }}
            >
              {t(`content.attachmentPreview.model.view.${name}`)}
            </button>
          ))}
        </div>
        {downloadEntry}
      </div>
      )}
    </div>
  );
}

function defaultMaterial(THREE: typeof import("three")): InstanceType<typeof THREE.MeshStandardMaterial> {
  return new THREE.MeshStandardMaterial({ color: 0x9ca3af, metalness: 0.1, roughness: 0.65, flatShading: true });
}

function applyBudgetToGeometry(geometry: { index: unknown; attributes: Record<string, { count: number }> }, budget: number) {
  const index = geometry.index as { count: number } | null;
  const triangles = index ? index.count / 3 : (geometry.attributes.position?.count ?? 0) / 3;
  if (triangles > budget) {
    throw new Error("triangle budget exceeded");
  }
}

function countGroupTriangles(
  object: { traverse: (cb: (child: object) => void) => void },
  budget: number,
) {
  let triangles = 0;
  object.traverse((raw) => {
    const child = raw as {
      isMesh?: boolean;
      geometry?: { index: unknown; attributes: Record<string, { count: number }> };
    };
    if (!child.isMesh || !child.geometry) return;
    const geo = child.geometry;
    const index = geo.index as { count: number } | null;
    triangles += index ? index.count / 3 : (geo.attributes.position?.count ?? 0) / 3;
  });
  if (triangles > budget) throw new Error("triangle budget exceeded");
}

/**
 * 场景装配与渲染循环。坐标合同：camera.up = +Z；GridHelper 旋到 XY 平面；
 * bbox 居中 + fit-to-view 距离 = 半径 * 2.5。
 */
function mountScene(
  THREE: typeof import("three"),
  OrbitControls: typeof import("three/examples/jsm/controls/OrbitControls.js").OrbitControls,
  object: InstanceType<typeof THREE.Object3D>,
  mount: HTMLDivElement | null,
  onUserInteract: () => void,
  initialView: ViewName,
): () => void {
  if (!mount) return () => undefined;

  const scene = new THREE.Scene();
  scene.background = new THREE.Color(0x101014);

  const box = new THREE.Box3().setFromObject(object);
  const center = box.getCenter(new THREE.Vector3());
  const size = box.getSize(new THREE.Vector3());
  const radius = Math.max(size.x, size.y, size.z) || 1;
  object.position.sub(center);
  scene.add(object);

  // Z-up 合同 + XY 底面网格（GridHelper 默认 XZ 平面，旋转到 XY）。
  scene.up.set(0, 0, 1);
  const grid = new THREE.GridHelper(Math.ceil(radius * 4), 20, 0x3f3f46, 0x27272a);
  grid.rotation.x = Math.PI / 2;
  grid.position.z = -size.z / 2 - radius * 0.05;
  scene.add(grid);

  const camera = new THREE.PerspectiveCamera(45, 1, radius / 100, radius * 20);
  camera.up.set(0, 0, 1);
  scene.add(new THREE.HemisphereLight(0xffffff, 0x303034, 1.1));
  const key = new THREE.DirectionalLight(0xffffff, 1.4);
  key.position.set(radius, -radius, radius * 1.5);
  scene.add(key);

  const renderer = new THREE.WebGLRenderer({ antialias: true });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
  mount.appendChild(renderer.domElement);

  const resize = () => {
    const width = mount.clientWidth || 1;
    const height = mount.clientHeight || 1;
    renderer.setSize(width, height, false);
    camera.aspect = width / height;
    camera.updateProjectionMatrix();
  };
  resize();
  const observer = new ResizeObserver(resize);
  observer.observe(mount);

  const controls = new OrbitControls(camera, renderer.domElement);
  controls.enableDamping = true;
  controls.addEventListener("start", onUserInteract);

  // 视图机位控制器：透视 orbit + 三视图正交位（合同值）。
  const controller = createViewController(THREE, camera, controls, radius, mount);
  controller.apply(initialView);

  let frame = 0;
  const loop = () => {
    frame = requestAnimationFrame(loop);
    controls.update();
    renderer.render(scene, controller.activeCamera());
  };
  loop();

  return () => {
    cancelAnimationFrame(frame);
    observer.disconnect();
    controls.dispose();
    controller.dispose();
    renderer.dispose();
    if (renderer.domElement.parentElement === mount) mount.removeChild(renderer.domElement);
    scene.traverse((child) => {
      const mesh = child as { geometry?: { dispose: () => void }; material?: { dispose: () => void } };
      mesh.geometry?.dispose();
      mesh.material?.dispose();
    });
  };
}

interface ViewController {
  apply(view: ViewName): void;
  activeCamera(): InstanceType<typeof import("three").Camera>;
  dispose(): void;
}

function createViewController(
  THREE: typeof import("three"),
  perspective: InstanceType<typeof import("three").PerspectiveCamera>,
  controls: InstanceType<typeof import("three/examples/jsm/controls/OrbitControls.js").OrbitControls>,
  radius: number,
  mount: HTMLDivElement,
): ViewController {
  const distance = radius * 2.5;
  const ortho = new THREE.OrthographicCamera(-radius * 1.4, radius * 1.4, radius * 1.4, -radius * 1.4, radius / 100, radius * 20);
  ortho.up.set(0, 0, 1);
  let active: ViewName = "orbit";

  return {
    apply(view: ViewName) {
      active = view;
      if (view === "orbit") {
        controls.enabled = true;
        perspective.position.set(distance * 0.7, -distance, distance * 0.8);
        controls.target.set(0, 0, 0);
        controls.update();
        return;
      }
      // 三视图正交机位（合同方向 × fit 距离）；正交位下禁用 orbit 交互。
      controls.enabled = false;
      const [dx, dy, dz] = VIEW_CONTRACT[view].dir;
      ortho.position.set(dx * distance, dy * distance, dz * distance);
      ortho.lookAt(0, 0, 0);
      ortho.updateProjectionMatrix();
    },
    activeCamera() {
      return active === "orbit" ? perspective : ortho;
    },
    dispose() {
      void mount;
      ortho.clear();
    },
  };
}
