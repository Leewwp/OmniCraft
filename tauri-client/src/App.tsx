import { useState, useEffect } from "react";
import { invoke } from "@tauri-apps/api/core";

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

enum Phase {
  Idle = "idle",
  Detecting = "detecting",
  Confirming = "confirming",
  Error = "error",
}

function App() {
  const [phase, setPhase] = useState<Phase>(Phase.Idle);
  const [deployParams, setDeployParams] = useState<DeployParams | null>(null);
  const [envInfo, setEnvInfo] = useState<EnvInfo | null>(null);
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

  // ── Render helpers ──────────────────────────────────────────────

  const phaseTitle = () => {
    switch (phase) {
      case Phase.Idle:
        return "欢迎使用 OmniCraft 桌面客户端";
      case Phase.Detecting:
        return "正在检测环境...";
      case Phase.Confirming:
        return "环境检测完成";
      case Phase.Error:
        return "出错";
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

            <div className="mb-4 rounded border border-border-muted bg-canvas-subtle p-3 text-sm text-fg-muted">
              一键部署通道已停用（安全整改：签名脚本执行链整体下线）。请通过
              Web 平台获取内容，外部 Agent 可经 REST/MCP 通道接入。
            </div>

            <div className="flex justify-end">
              <button
                onClick={() => setPhase(Phase.Idle)}
                className="rounded-md border border-border px-4 py-2 text-sm transition-colors hover:bg-canvas-subtle"
              >
                返回
              </button>
            </div>
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
              }}
              className="mt-6 rounded-md border border-border px-4 py-2 text-sm transition-colors hover:bg-canvas-subtle"
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
