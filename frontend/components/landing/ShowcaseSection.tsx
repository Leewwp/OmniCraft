"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import {
  buildIpShowcase,
  buildWorkShowcase,
  type ShowcaseIp,
  type ShowcaseWork,
} from "@/lib/landing-showcase";

// 精选陈列（票 #853 / spec Q7+§12.2，零新后端接口）：
// - 作品：公开 GET /contents 两区候选（fanwork/original × hot 主序 + newest 补槽）；
// - IP：公开 GET /ips（most_contents 主序 → newest 补槽）；
// - 固定 10 槽、缺封面过滤、按 id 去重、保持服务端顺序（不发明热度权重）；
// - 补完 <4 项该子区整段隐藏（宁可少一段不留空壳）；
// - 取数失败/空数据整段隐藏不白屏。

interface WorkCandidate {
  id: number;
  title?: string;
  zone?: string;
  cover_image_url?: string | null;
}

interface IpCandidate {
  id: number;
  name?: string;
  cover_url?: string | null;
}

const FETCH_PAGE_SIZE = 12;

async function fetchJson<T, R>(url: string, pick: (data: T) => R): Promise<R | null> {
  try {
    const res = await fetch(url, { cache: "no-store" });
    if (!res.ok) return null;
    return pick((await res.json()) as T);
  } catch {
    return null;
  }
}

export function ShowcaseSection({ apiBase }: { apiBase: string }) {
  const t = useTranslations("landing");
  const [works, setWorks] = useState<ShowcaseWork[]>([]);
  const [ips, setIps] = useState<ShowcaseIp[]>([]);
  const [settled, setSettled] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const [fanworkHot, originalHot, fanworkNewest, originalNewest, ipPrimary, ipNewest] =
        await Promise.all([
          fetchJson<{ contents?: WorkCandidate[] }, WorkCandidate[]>(
            `${apiBase}/contents?zone=fanwork&sort=hot&time_range=all&page=1&page_size=${FETCH_PAGE_SIZE}`,
            (d) => (Array.isArray(d?.contents) ? d.contents : []),
          ),
          fetchJson<{ contents?: WorkCandidate[] }, WorkCandidate[]>(
            `${apiBase}/contents?zone=original&sort=hot&time_range=all&page=1&page_size=${FETCH_PAGE_SIZE}`,
            (d) => (Array.isArray(d?.contents) ? d.contents : []),
          ),
          fetchJson<{ contents?: WorkCandidate[] }, WorkCandidate[]>(
            `${apiBase}/contents?zone=fanwork&sort=newest&page=1&page_size=${FETCH_PAGE_SIZE}`,
            (d) => (Array.isArray(d?.contents) ? d.contents : []),
          ),
          fetchJson<{ contents?: WorkCandidate[] }, WorkCandidate[]>(
            `${apiBase}/contents?zone=original&sort=newest&page=1&page_size=${FETCH_PAGE_SIZE}`,
            (d) => (Array.isArray(d?.contents) ? d.contents : []),
          ),
          fetchJson<{ ips?: IpCandidate[] }, IpCandidate[]>(
            `${apiBase}/ips?sort=most_contents&page=1&page_size=${FETCH_PAGE_SIZE}`,
            (d) => (Array.isArray(d?.ips) ? d.ips : []),
          ),
          fetchJson<{ ips?: IpCandidate[] }, IpCandidate[]>(
            `${apiBase}/ips?sort=newest&page=1&page_size=${FETCH_PAGE_SIZE}`,
            (d) => (Array.isArray(d?.ips) ? d.ips : []),
          ),
        ]);
      if (cancelled) return;
      // 任一请求失败（null）按空候选处理 → 对应子区可能整段隐藏，不白屏。
      setWorks(
        buildWorkShowcase({
          fanworkHot: fanworkHot ?? [],
          originalHot: originalHot ?? [],
          fanworkNewest: fanworkNewest ?? [],
          originalNewest: originalNewest ?? [],
        }),
      );
      setIps(
        buildIpShowcase({
          primary: ipPrimary ?? [],
          newest: ipNewest ?? [],
        }),
      );
      setSettled(true);
    })();
    return () => {
      cancelled = true;
    };
  }, [apiBase]);

  if (!settled) return null;
  if (works.length === 0 && ips.length === 0) return null;

  return (
    <>
      {works.length > 0 && (
        <div className="works" data-testid="showcase-works">
          {works.map((work) => (
            <Link key={work.id} href={`/content/${work.id}`} className="work" data-testid="showcase-work">
              <div className="cv">
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img src={work.coverUrl} alt="" loading="lazy" />
              </div>
              <div className="wb">
                <div className="wt">{work.title}</div>
                <div className="wm">
                  <span className={work.zone === "original" ? "zn z-o" : "zn z-f"}>
                    {work.zone === "original" ? t("showcase.zoneOriginal") : t("showcase.zoneFanwork")}
                  </span>
                </div>
              </div>
            </Link>
          ))}
        </div>
      )}
      {ips.length > 0 && (
        <div className="iprow" data-testid="showcase-ips">
          {ips.map((ip) => (
            <Link key={ip.id} href={`/ip/${ip.id}`} className="ip" data-testid="showcase-ip">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={ip.coverUrl} alt="" loading="lazy" />
              <span>{ip.name}</span>
            </Link>
          ))}
        </div>
      )}
      <div className="more">
        <Link href="/ips">{t("showcase.moreIps")}</Link>
      </div>
    </>
  );
}
