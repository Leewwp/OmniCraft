"use client";

import { useEffect, useRef, useCallback } from "react";
import { useTranslations } from "next-intl";
import { fetchPublicConfig } from "@/lib/public-config";

declare global {
  interface Window {
    AliyunCaptchaConfig?: {
      region?: string;
      prefix?: string;
    };
    initAliyunCaptcha?: (options: AliyunCaptchaOptions) => void;
  }
}

interface CaptchaWidgetProps {
  onToken: (token: string) => void;
  onError?: (error: string) => void;
}

export default function CaptchaWidget({ onToken, onError }: CaptchaWidgetProps) {
  const t = useTranslations();
  const containerRef = useRef<HTMLDivElement>(null);
  const initialized = useRef(false);
  const elementId = useRef(`aliyun-captcha-${Math.random().toString(36).slice(2)}`);

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
          window.AliyunCaptchaConfig = {
            region: config.captcha.region || "cn",
            prefix: config.captcha.prefix,
          };
          const script = document.createElement("script");
          script.id = scriptId;
          script.src = "https://o.alicdn.com/captcha-frontend/aliyunCaptcha/AliyunCaptcha.js";
          script.async = true;
          document.head.appendChild(script);
          await new Promise<void>((resolve, reject) => {
            script.onload = () => resolve();
            script.onerror = () => reject(new Error("Failed to load captcha SDK"));
          });
        }

        if (!window.initAliyunCaptcha) {
          onError?.(t("auth.captchaFailed"));
          return;
        }

        window.initAliyunCaptcha({
          SceneId: config.captcha.scene_id,
          prefix: config.captcha.prefix,
          mode: "embed",
          element: `#${elementId.current}`,
          language: config.captcha.region === "cn" ? "cn" : "en",
          captchaVerifyCallback: async (captchaVerifyParam: string) => {
            onToken(captchaVerifyParam);
            return {
              captchaResult: true,
              bizResult: true,
            };
          },
          onBizResultCallback: () => undefined,
          getInstance: () => undefined,
          slideStyle: {
            width: 320,
            height: 40,
          },
        });
      }
    } catch {
      initialized.current = false;
      onError?.(t("auth.captchaFailed"));
    }
  }, [onToken, onError, t]);

  useEffect(() => {
    initCaptcha();
  }, [initCaptcha]);

  return <div id={elementId.current} ref={containerRef} className="captcha-widget" />;
}

interface AliyunCaptchaOptions {
  SceneId: string;
  prefix: string;
  mode: "embed" | "popup";
  element: string;
  language: "cn" | "en";
  captchaVerifyCallback: (captchaVerifyParam: string) => Promise<{
    captchaResult: boolean;
    bizResult: boolean;
  }>;
  onBizResultCallback: () => void;
  getInstance: (instance: unknown) => void;
  slideStyle: {
    width: number;
    height: number;
  };
}
