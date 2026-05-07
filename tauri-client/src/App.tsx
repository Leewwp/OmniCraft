import { useState, useEffect } from "react";
import { invoke } from "@tauri-apps/api/core";

function App() {
  const [deployInfo, setDeployInfo] = useState<{
    contentId?: string;
    token?: string;
  } | null>(null);

  useEffect(() => {
    void (async () => {
      try {
        const params = await invoke<{ content_id?: string; token?: string }>(
          "get_deploy_params"
        );
        if (params.content_id) {
          setDeployInfo({
            contentId: params.content_id,
            token: params.token,
          });
        }
      } catch {
        // URL Scheme params not available — normal standalone launch
      }
    })();
  }, []);

  return (
    <div className="min-h-screen bg-canvas-default text-fg-default">
      <header className="flex items-center justify-between border-b border-border px-6 py-3">
        <h1 className="text-lg font-semibold tracking-tight">
          OmniCraft 万象工坊
        </h1>
        <span className="text-xs text-muted-foreground">PC Client</span>
      </header>

      <main className="mx-auto max-w-2xl px-4 py-8 text-center">
        {deployInfo ? (
          <div className="space-y-4 rounded-md border border-border bg-card p-6">
            <h2 className="text-xl font-bold">内容部署</h2>
            <p className="text-sm text-muted-foreground">
              正在准备部署内容 #{deployInfo.contentId}
            </p>
            <p className="text-xs text-muted-foreground">
              部署流程将在后续版本中实现（Task 35）
            </p>
          </div>
        ) : (
          <div className="space-y-4 rounded-md border border-border bg-card p-12">
            <h2 className="text-xl font-bold">欢迎使用 OmniCraft 桌面客户端</h2>
            <p className="text-sm text-muted-foreground">
              从浏览器中点击「一键部署」以开始使用，或直接浏览 Web 平台。
            </p>
          </div>
        )}
      </main>
    </div>
  );
}

export default App;
