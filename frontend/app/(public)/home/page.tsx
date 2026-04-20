import Link from "next/link";
import { Brush, Sparkles, Users, Layers } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export default function HomePage() {
  return (
    <div className="flex flex-col">
      {/* Hero */}
      <section className="flex flex-col items-center justify-center gap-6 px-4 py-24 text-center">
        <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/10">
          <Brush className="h-8 w-8 text-primary" />
        </div>
        <div className="flex flex-col gap-3">
          <h1 className="text-4xl font-bold tracking-tight sm:text-5xl">
            万象工坊
          </h1>
          <p className="max-w-xl text-lg text-muted-foreground leading-relaxed">
            以 IP 二创内容聚合为核心，Agent 自动化为增值能力，
            <br className="hidden sm:block" />
            GitHub 式 PR 协同为护城河的全民创意分享平台
          </p>
        </div>
        <div className="flex flex-wrap items-center justify-center gap-3">
          <Link href="/register" className={cn(buttonVariants({ size: "lg" }))}>
            立即加入
          </Link>
          <Link href="/login" className={cn(buttonVariants({ variant: "outline", size: "lg" }))}>
            登录账号
          </Link>
        </div>
      </section>

      {/* Features */}
      <section className="border-t border-border bg-muted/30 px-4 py-16">
        <div className="mx-auto grid max-w-4xl gap-8 sm:grid-cols-3">
          <div className="flex flex-col items-center gap-3 text-center">
            <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10">
              <Layers className="h-6 w-6 text-primary" />
            </div>
            <h3 className="font-semibold">IP 二创聚合</h3>
            <p className="text-sm text-muted-foreground leading-relaxed">
              围绕热门 IP 聚合二次创作，图文、视频、音乐、Mod 一网打尽
            </p>
          </div>
          <div className="flex flex-col items-center gap-3 text-center">
            <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10">
              <Sparkles className="h-6 w-6 text-primary" />
            </div>
            <h3 className="font-semibold">Agent 自动部署</h3>
            <p className="text-sm text-muted-foreground leading-relaxed">
              一键部署 Mod 到游戏目录，AI 辅助创作与智能推荐
            </p>
          </div>
          <div className="flex flex-col items-center gap-3 text-center">
            <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10">
              <Users className="h-6 w-6 text-primary" />
            </div>
            <h3 className="font-semibold">PR 协同创作</h3>
            <p className="text-sm text-muted-foreground leading-relaxed">
              GitHub 式协同修改，版本管理，众裁审核，共同完善作品
            </p>
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="flex flex-col items-center gap-4 px-4 py-16 text-center">
        <p className="text-muted-foreground text-sm">平台正在建设中，敬请期待更多功能</p>
        <div className="flex items-center gap-2 text-xs text-muted-foreground/60">
          <span>后端 API</span>
          <span className="h-1 w-1 rounded-full bg-green-500"></span>
          <span className="text-green-600 font-medium">运行中</span>
          <span className="mx-2">·</span>
          <Link href="/login" className="hover:text-foreground transition-colors">
            登录
          </Link>
          <span>·</span>
          <Link href="/register" className="hover:text-foreground transition-colors">
            注册
          </Link>
        </div>
      </section>
    </div>
  );
}
