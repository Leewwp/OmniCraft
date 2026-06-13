"use client";

import { useEffect, useRef, useCallback, useState } from "react";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
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

export interface CaptchaWidgetProps {
  onToken: (token: string) => void;
  onError?: (error: string) => void;
  containerId?: string;
  buttonId?: string;
}

type CaptchaStatus = "loading" | "ready" | "verified" | "failed";

interface CaptchaVerifyResponse {
  captcha_result: boolean;
  captcha_token: string;
}

export function CaptchaWidget({ onToken, onError, containerId, buttonId = "captcha-submit-button" }: CaptchaWidgetProps) {
  const t = useTranslations();
  const initialized = useRef(false);
  const generatedElementId = useRef(`aliyun-captcha-${Math.random().toString(36).slice(2)}`);
  const elementId = containerId ?? generatedElementId.current;
  const [status, setStatus] = useState<CaptchaStatus>("loading");
  const [message, setMessage] = useState(t("auth.captchaLoading"));

  const initCaptcha = useCallback(async () => {
    if (initialized.current) return;
    initialized.current = true;
    setStatus("loading");
    setMessage(t("auth.captchaLoading"));

    try {
      const config = await fetchPublicConfig();
      if (config.captcha.provider === "bypass") {
        setStatus("verified");
        setMessage(t("auth.captchaVerified"));
        onToken("bypass");
        return;
      }

      if (config.captcha.provider === "aliyun_v2") {
        if (!config.captcha.prefix || !config.captcha.scene_id) {
          setStatus("failed");
          setMessage(t("auth.captchaFailed"));
          onError?.(t("auth.captchaFailed"));
          return;
        }

        const scriptId = "aliyun-captcha-sdk";
        window.AliyunCaptchaConfig = {
          region: config.captcha.region || "cn",
          prefix: config.captcha.prefix,
        };

        if (!document.getElementById(scriptId)) {
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
          setStatus("failed");
          setMessage(t("auth.captchaFailed"));
          onError?.(t("auth.captchaFailed"));
          return;
        }

        setStatus("ready");
        setMessage(t("auth.captchaReady"));
        window.initAliyunCaptcha({
          SceneId: config.captcha.scene_id,
          prefix: config.captcha.prefix,
          mode: "embed",
          element: `#${elementId}`,
          button: `#${buttonId}`,
          language: config.captcha.region === "cn" ? "cn" : "en",
          captchaVerifyCallback: async (captchaVerifyParam: string) => {
            try {
              const verifyResult = await api.post<CaptchaVerifyResponse>("/api/v1/captcha/verify", {
                captcha_verify_param: captchaVerifyParam,
              });
              if (!verifyResult.captcha_result || !verifyResult.captcha_token) {
                throw new Error("captcha verification failed");
              }
              onToken(verifyResult.captcha_token);
              setStatus("verified");
              setMessage(t("auth.captchaVerified"));
              return {
                captchaResult: true,
                bizResult: true,
              };
            } catch {
              onToken("");
              setStatus("failed");
              setMessage(t("auth.captchaFailed"));
              onError?.(t("auth.captchaFailed"));
              return {
                captchaResult: false,
                bizResult: false,
              };
            }
          },
          onBizResultCallback: () => {
            setStatus("verified");
            setMessage(t("auth.captchaVerified"));
          },
          getInstance: () => undefined,
          slideStyle: {
            width: 320,
            height: 40,
          },
          immediate: true,
          autoRefresh: false,
        });
        return;
      }

      setStatus("failed");
      setMessage(t("auth.captchaFailed"));
      onError?.(t("auth.captchaFailed"));
    } catch {
      initialized.current = false;
      setStatus("failed");
      setMessage(t("auth.captchaFailed"));
      onError?.(t("auth.captchaFailed"));
    }
  }, [buttonId, elementId, onToken, onError, t]);

  useEffect(() => {
    initCaptcha();
  }, [initCaptcha]);

  return (
    <div className="flex flex-col gap-2">
      <div id={elementId} className="captcha-widget min-h-10" />
      <p
        className={status === "failed" ? "text-xs text-destructive" : "text-xs text-muted-foreground"}
        role={status === "failed" ? "alert" : "status"}
      >
        {message}
      </p>
    </div>
  );
}

export default CaptchaWidget;

interface AliyunCaptchaOptions {
  SceneId: string;
  prefix: string;
  mode: "embed" | "popup";
  element: string;
  button: string;
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
  immediate: boolean;
  autoRefresh: boolean;
}
