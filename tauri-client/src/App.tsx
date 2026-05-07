import { useState, useEffect, useCallback } from "react";
import { invoke } from "@tauri-apps/api/core";
import { confirm } from "@tauri-apps/plugin-dialog";
import {
  isPermissionGranted,
  requestPermission,
  sendNotification,
} from "@tauri-apps/plugin-notification";

interface DeployParams {
  content_id: string;
  token?: string;
}

interface EnvInfo {
  steam_paths: string[];
  platform: string;
  home_dir: string;
  appdata_dir: string;
}

interface DeployAction {
  action: string;
  payload: Record<string, string>;
}

interface DeployScript {
  content_id: string;
  actions: DeployAction[];
}

interface SignedDeployScript {
  script: DeployScript;
  signature: string;
}

enum Phase {
  Idle = "idle",
  Detecting = "detecting",
  Confirming = "confirming",
  Executing = "executing",
  Done = "done",
  Error = "error",
}

const API_BASE = "http://localhost:8080/api/v1";

function App() {
  const [phase, setPhase] = useState<Phase>(Phase.Idle);
  const [deployParams, setDeployParams] = useState<DeployParams | null>(null);
  const [envInfo, setEnvInfo] = useState<EnvInfo | null>(null);
  const [actions, setActions] = useState<DeployAction[]>([]);
  const [currentStep, setCurrentStep] = useState(-1);
  const [stepStatus, setStepStatus] = useState<string[]>([]);
  const [error, setError] = useState("");

  // ── Phase 1: Read URL scheme params ──────────────────────────────
  useEffect(() => {
    void (async () => {
      try {
        const params = await invoke<DeployParams>("get_deploy_params");
        if (params?.content_id) {
          setDeployParams(params);
          setPhase(Phase.Detecting);
        }
      } catch {
        // standalone launch — no deploy params
      }
    })();
  }, []);

  // ── Phase 2: Environment detection ──────────────────────────────
  useEffect(() => {
    if (phase !== Phase.Detecting || !deployParams) return;
    void (async () => {
      try {
        const info = await invoke<EnvInfo>("detect_environment");
        setEnvInfo(info);
        setPhase(Phase.Confirming);
      } catch (e) {
        setError(String(e));
        setPhase(Phase.Error);
      }
    })();
  }, [phase, deployParams]);

  // ── Phase 3: Confirmation ─────────────────────────────────────────
  const handleConfirm = useCallback(async () => {
    if (!deployParams) return;
    setPhase(Phase.Confirming);

    const confirmed = await confirm(
      "检测到以下环境信息：\n\n" +
        `Steam 路径：${envInfo?.steam_paths.join(", ") || "未检测到"}\n` +
        `平台：${envInfo?.platform}\n\n` +
        "确认继续部署？",
      { title: "一键部署确认", kind: "warning" }
    );

    if (!confirmed) {
      setPhase(Phase.Idle);
      return;
    }

    setPhase(Phase.Executing);

    try {
      // Fetch signed script from Go backend
      const token = deployParams.token || "";
      const res = await fetch(`${API_BASE}/agent/script/${deployParams.content_id}`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.message || `API error ${res.status}`);
      }
      const signed: SignedDeployScript = await res.json();
      const { script, signature } = signed;

      // Verify signature
      await invoke("verify_script_signature", {
        scriptJson: JSON.stringify(script),
        signature,
      });

      setActions(script.actions);
      const statuses = script.actions.map(() => "⏳");
      setStepStatus(statuses);

      // ── Execute actions sequentially ──
      for (let i = 0; i < script.actions.length; i++) {
        setCurrentStep(i);
        const action = script.actions[i];
        try {
          await invoke(action.action, action.payload);
          statuses[i] = "✅";
        } catch (e) {
          statuses[i] = `❌ ${String(e).slice(0, 80)}`;
          throw e;
        }
        setStepStatus([...statuses]);
      }

      setCurrentStep(script.actions.length);
      setPhase(Phase.Done);

      // Desktop notification
      let perm = await isPermissionGranted();
      if (!perm) {
        const request = await requestPermission();
        perm = request === "granted";
      }
      if (perm) {
        sendNotification({
          title: "OmniCraft 部署完成",
          body: `内容 #${deployParams.content_id} 部署成功`,
        });
      }
    } catch (e) {
      setError(String(e));
      setPhase(Phase.Error);
    }
  }, [deployParams, envInfo]);

  // ── Render helpers ──────────────────────────────────────────────

  const actionLabel = (a: DeployAction) => {
    switch (a.action) {
      case "download_file":
        return `下载 ${a.payload.dest || a.payload.url?.slice(-20)}`;
      case "extract_archive":
        return `解压 ${a.payload.path} → ${a.payload.dest}`;
      case "move_file":
        return `移动 ${a.payload.source} → ${a.payload.dest}`;
      case "create_dir":
        return `创建目录 ${a.payload.dir_path}`;
      case "read_config":
        return `读取配置 ${a.payload.config_path}`;
      case "write_config":
        return `写入配置 ${a.payload.config_path}`;
      default:
        return a.action;
    }
  };

  const phaseTitle = () => {
    switch (phase) {
      case Phase.Idle:
        return "欢迎使用 OmniCraft 桌面客户端";
      case Phase.Detecting:
        return "正在检测环境...";
      case Phase.Confirming:
        return "环境检测完成";
      case Phase.Executing:
        return "正在部署...";
      case Phase.Done:
        return "部署完成";
      case Phase.Error:
        return "部署出错";
    }
  };

  return (
    <div className="min-h-screen bg-canvas-default text-fg-default">
      <header className="flex items-center justify-between border-b border-border px-6 py-3">
        <h1 className="text-lg font-semibold tracking-tight">
          OmniCraft 万象工坊
        </h1>
        <span className="text-xs text-muted-foreground">PC Client</span>
      </header>

      <main className="mx-auto max-w-2xl px-4 py-8">
        {/* ── Idle state ── */}
        {phase === Phase.Idle && (
          <div className="rounded-md border border-border bg-card p-12 text-center">
            <h2 className="mb-3 text-xl font-bold">{phaseTitle()}</h2>
            <p className="text-sm text-muted-foreground">
              从浏览器中点击「一键部署」以开始使用，或直接浏览 Web 平台。
            </p>
            {deployParams && (
              <button
                onClick={() => setPhase(Phase.Detecting)}
                className="mt-6 rounded-md border border-border px-4 py-2 text-sm transition-colors hover:bg-canvas-subtle"
              >
                重新检测环境
              </button>
            )}
          </div>
        )}

        {/* ── Detecting ── */}
        {phase === Phase.Detecting && (
          <div className="rounded-md border border-border bg-card p-12 text-center">
            <h2 className="mb-3 text-xl font-bold">{phaseTitle()}</h2>
            <div className="inline-block h-8 w-8 animate-spin rounded-full border-2 border-border border-t-accent-default" />
            <p className="mt-4 text-sm text-muted-foreground">
              正在扫描本地游戏安装路径...
            </p>
          </div>
        )}

        {/* ── Confirming ── */}
        {phase === Phase.Confirming && envInfo && (
          <div className="space-y-4">
            <div className="rounded-md border border-border bg-card p-6">
              <h2 className="mb-4 text-xl font-bold">{phaseTitle()}</h2>

              <div className="mb-3 text-sm">
                <span className="text-muted-foreground">内容 ID：</span>
                <span className="font-mono">{deployParams?.content_id}</span>
              </div>

              <div className="mb-3 text-sm">
                <span className="text-muted-foreground">平台：</span>
                <span>{envInfo.platform}</span>
              </div>

              <div className="mb-3 text-sm">
                <span className="text-muted-foreground">Steam 路径：</span>
                {envInfo.steam_paths.length > 0 ? (
                  <ul className="mt-1 list-inside list-disc">
                    {envInfo.steam_paths.map((p) => (
                      <li key={p} className="font-mono text-xs">
                        {p}
                      </li>
                    ))}
                  </ul>
                ) : (
                  <span className="text-fg-subtle">未检测到 Steam</span>
                )}
              </div>

              <div className="mb-4 text-sm">
                <span className="text-muted-foreground">目标目录：</span>
                <span className="font-mono text-xs">{envInfo.home_dir}/.omnicraft</span>
              </div>
            </div>

            <div className="flex justify-end gap-3">
              <button
                onClick={() => setPhase(Phase.Idle)}
                className="rounded-md border border-border px-4 py-2 text-sm transition-colors hover:bg-canvas-subtle"
              >
                取消
              </button>
              <button
                onClick={() => { handleConfirm(); }}
                className="rounded-md border border-accent-default bg-accent-subtle px-4 py-2 text-sm font-medium text-accent-default transition-colors hover:opacity-80"
              >
                确认并部署
              </button>
            </div>
          </div>
        )}

        {/* ── Executing ── */}
        {phase === Phase.Executing && (
          <div className="rounded-md border border-border bg-card p-6">
            <h2 className="mb-4 text-xl font-bold">{phaseTitle()}</h2>
            <ul className="space-y-2">
              {actions.map((a, i) => (
                <li
                  key={i}
                  className={`flex items-center gap-3 rounded border px-3 py-2 text-sm ${
                    i === currentStep
                      ? "border-accent-default bg-accent-subtle"
                      : i < currentStep
                        ? "border-border-muted text-fg-muted"
                        : "border-border text-fg-subtle"
                  }`}
                >
                  <span className="font-mono text-xs w-6 text-center">
                    {stepStatus[i]}
                  </span>
                  <span>{actionLabel(a)}</span>
                </li>
              ))}
            </ul>
            {currentStep >= actions.length && (
              <p className="mt-4 text-center text-sm text-accent-default">
                全部操作执行完毕
              </p>
            )}
          </div>
        )}

        {/* ── Done ── */}
        {phase === Phase.Done && (
          <div className="rounded-md border border-border bg-card p-12 text-center">
            <h2 className="mb-3 text-xl font-bold text-accent-default">
              {phaseTitle()}
            </h2>
            <p className="text-sm text-muted-foreground">
              内容已成功部署到本地，现在可以启动对应游戏或应用使用。
            </p>
            <button
              onClick={() => {
                setPhase(Phase.Idle);
                setDeployParams(null);
                setEnvInfo(null);
                setActions([]);
                setCurrentStep(-1);
                setError("");
              }}
              className="mt-6 rounded-md border border-border px-4 py-2 text-sm transition-colors hover:bg-canvas-subtle"
            >
              完成
            </button>
          </div>
        )}

        {/* ── Error ── */}
        {phase === Phase.Error && (
          <div className="rounded-md border border-border bg-card p-12 text-center">
            <h2 className="mb-3 text-xl font-bold text-fg-default">
              {phaseTitle()}
            </h2>
            <p className="mb-4 rounded border border-border-muted bg-canvas-subtle p-3 font-mono text-xs text-left whitespace-pre-wrap">
              {error}
            </p>
            <button
              onClick={() => {
                setPhase(Phase.Idle);
                setError("");
                setActions([]);
                setCurrentStep(-1);
                setStepStatus([]);
              }}
              className="rounded-md border border-border px-4 py-2 text-sm transition-colors hover:bg-canvas-subtle"
            >
              返回
            </button>
          </div>
        )}
      </main>
    </div>
  );
}

export default App;
