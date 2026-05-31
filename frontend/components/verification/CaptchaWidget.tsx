"use client";

import { useEffect, useRef, useCallback } from "react";
import { useTranslations } from "next-intl";
import { fetchPublicConfig } from "@/lib/public-config";

interface CaptchaWidgetProps {
  onToken: (token: string) => void;
  onError?: (error: string) => void;
}

export default function CaptchaWidget({ onToken, onError }: CaptchaWidgetProps) {
  const t = useTranslations();
  const containerRef = useRef<HTMLDivElement>(null);
  const initialized = useRef(false);

  const initCaptcha = useCallback(async () => {
    if (initialized.current) return;
    initialized.current = true;

    try {
      const config = await fetchPublicConfig();
      if (config.captcha.provider === "bypass" || !config.captcha.provider) {
        onToken("bypass");
        return;
      }

      if (config.captcha.provider === "aliyun_v2") {
        const scriptId = "aliyun-captcha-sdk";
        if (!document.getElementById(scriptId)) {
          const script = document.createElement("script");
          script.id = scriptId;
          script.src = `https://o.alicdn.com/captcha-frontend/aliyunCaptcha/aliyun-captcha.js`;
          script.async = true;
          document.head.appendChild(script);
          await new Promise<void>((resolve, reject) => {
            script.onload = () => resolve();
            script.onerror = () => reject(new Error("Failed to load captcha SDK"));
          });
        }

        const w = window as unknown as Record<string, unknown>;
        const AWSC = w.AWSC as AliyunCaptchaSDK | undefined;
        if (!AWSC) {
          onError?.("Captcha SDK not available");
          return;
        }

        AWSC.use("nc", function (_: unknown, module: AliyunCaptchaModule) {
          const nc = module.init({
            appkey: config.captcha.prefix,
            scene: config.captcha.scene_id,
            renderTo: containerRef.current,
            success: function (data: { sessionId?: string; sig?: string; token?: string; ncsig?: string }) {
              const tokenStr = [data.sessionId, data.sig, data.token, data.ncsig].filter(Boolean).join("|");
              onToken(tokenStr);
            },
            fail: function () {
              onError?.(t("auth.captchaFailed"));
            },
          });
          nc.reset();
        });
      }
    } catch {
      onToken("bypass");
    }
  }, [onToken, onError, t]);

  useEffect(() => {
    initCaptcha();
  }, [initCaptcha]);

  return <div ref={containerRef} className="captcha-widget" />;
}

interface AliyunCaptchaSDK {
  use: (type: string, callback: (mode: unknown, module: AliyunCaptchaModule) => void) => void;
}

interface AliyunCaptchaModule {
  init: (options: {
    appkey: string;
    scene: string;
    renderTo: HTMLDivElement | null;
    success: (data: { sessionId?: string; sig?: string; token?: string; ncsig?: string }) => void;
    fail: () => void;
  }) => { reset: () => void };
}
