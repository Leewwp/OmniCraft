"use client";

import { useState, FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Brush, Eye, EyeOff, Loader2 } from "lucide-react";
import { api, ApiRequestError } from "@/lib/api";
import { saveTokens } from "@/lib/auth";
import { useAuth } from "@/contexts/AuthContext";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface RegisterResponse {
  user: {
    id: number;
    email: string;
    username: string;
    role: string;
  };
  tokens: {
    access_token: string;
    refresh_token: string;
  };
}

export default function RegisterPage() {
  const router = useRouter();
  const { login } = useAuth();

  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  function validate(): boolean {
    const newErrors: Record<string, string> = {};
    if (username.length < 2) newErrors.username = "用户名至少 2 个字符";
    if (username.length > 64) newErrors.username = "用户名不超过 64 个字符";
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) newErrors.email = "请输入有效的邮箱地址";
    if (password.length < 6) newErrors.password = "密码至少 6 位";
    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!validate()) return;

    setErrors({});
    setIsLoading(true);

    try {
      const data = await api.post<RegisterResponse>("/api/v1/auth/register", {
        username,
        email,
        password,
      });
      saveTokens(data.tokens.access_token, data.tokens.refresh_token);
      await login(email, password);
      router.push("/");
    } catch (err) {
      if (err instanceof ApiRequestError) {
        if (err.code === "USER_EXISTS") {
          setErrors({ email: "该邮箱已被注册" });
        } else if (err.code === "USERNAME_TAKEN") {
          setErrors({ username: "该用户名已被占用" });
        } else if (err.code === "VALIDATION_ERROR") {
          setErrors({ general: "请检查输入内容格式" });
        } else {
          setErrors({ general: err.message || "注册失败，请稍后重试" });
        }
      } else {
        setErrors({ general: "网络错误，请检查连接后重试" });
      }
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center px-4 py-12">
      <div className="w-full max-w-sm">
        {/* Logo */}
        <div className="mb-8 flex flex-col items-center gap-2">
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10">
            <Brush className="h-6 w-6 text-primary" />
          </div>
          <h1 className="text-2xl font-semibold tracking-tight">加入万象工坊</h1>
          <p className="text-sm text-muted-foreground">创建你的创作者账号</p>
        </div>

        {/* Form */}
        <div className="rounded-lg border border-border bg-card p-6 shadow-none">
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="username">用户名</Label>
              <Input
                id="username"
                type="text"
                placeholder="创作者昵称"
                autoComplete="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                disabled={isLoading}
                className={errors.username ? "border-destructive" : ""}
              />
              {errors.username && (
                <p className="text-xs text-destructive">{errors.username}</p>
              )}
            </div>

            <div className="flex flex-col gap-1.5">
              <Label htmlFor="email">邮箱</Label>
              <Input
                id="email"
                type="email"
                placeholder="you@example.com"
                autoComplete="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                disabled={isLoading}
                className={errors.email ? "border-destructive" : ""}
              />
              {errors.email && (
                <p className="text-xs text-destructive">{errors.email}</p>
              )}
            </div>

            <div className="flex flex-col gap-1.5">
              <Label htmlFor="password">密码</Label>
              <div className="relative">
                <Input
                  id="password"
                  type={showPassword ? "text" : "password"}
                  placeholder="至少 6 位"
                  autoComplete="new-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  disabled={isLoading}
                  className={`pr-10 ${errors.password ? "border-destructive" : ""}`}
                />
                <button
                  type="button"
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                  onClick={() => setShowPassword(!showPassword)}
                  tabIndex={-1}
                >
                  {showPassword ? (
                    <EyeOff className="h-4 w-4" />
                  ) : (
                    <Eye className="h-4 w-4" />
                  )}
                </button>
              </div>
              {errors.password && (
                <p className="text-xs text-destructive">{errors.password}</p>
              )}
            </div>

            {errors.general && (
              <p className="text-sm text-destructive" role="alert">
                {errors.general}
              </p>
            )}

            <Button type="submit" className="w-full mt-1" disabled={isLoading}>
              {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              创建账号
            </Button>
          </form>
        </div>

        <p className="mt-4 text-center text-sm text-muted-foreground">
          已有账号？{" "}
          <Link href="/login" className="font-medium text-primary hover:underline">
            立即登录
          </Link>
        </p>
      </div>
    </div>
  );
}
