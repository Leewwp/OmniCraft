"use client";

import { useTranslations } from "next-intl";

// 特色玩法 2×2 四机制卡（R5 Q20）：每卡 = 机制名 + 一句话 + 底部循环 CSS
// 动画演示使用场景。机制名/一句话走 landing.mechanics.*，动画演示内文
// 走 landing.demo.mechanics.* —— 演示内文已双译（§12.2）。

export function MechanicsGrid() {
  const t = useTranslations("landing");
  const d = useTranslations("landing.demo.mechanics");

  return (
    <div className="mgrid" data-testid="landing-mechanics">
      {/* A 二创投稿（提交 → 审阅 → 合入） */}
      <div className="mcard">
        <div className="mh">
          <span className="mtag">{t("mechanics.pr.tag")}</span>
          <h3>{t("mechanics.pr.title")}</h3>
        </div>
        <div className="msub">{t("mechanics.pr.sub")}</div>
        <div className="md md-pr">
          <div className="pr-steps">
            <div className="pr-st pr-s1">
              <i>1</i>
              <div>
                <b>{d("prStep1t")}</b>
                <span>{d("prStep1s")}</span>
              </div>
            </div>
            <div className="pr-st pr-s2">
              <i>2</i>
              <div>
                <b>{d("prStep2t")}</b>
                <span>{d("prStep2s")}</span>
              </div>
            </div>
            <div className="pr-st pr-s3">
              <i>3</i>
              <div>
                <b>{d("prStep3t")}</b>
                <span>{d("prStep3s")}</span>
              </div>
            </div>
          </div>
          <div className="pr-file">
            <div className="pr-fname">{d("prFile")}</div>
            <div className="pr-body">
              <div className="pr-ln ctx">{d("prLine1")}</div>
              <div className="pr-ln add">{d("prLine2")}</div>
              <div className="pr-ln add">{d("prLine3")}</div>
            </div>
            <div className="pr-merged">{d("prMerged")}</div>
          </div>
        </div>
      </div>

      {/* B 赛博判官（投票 → 判定） */}
      <div className="mcard">
        <div className="mh">
          <span className="mtag">{t("mechanics.judge.tag")}</span>
          <h3>{t("mechanics.judge.title")}</h3>
        </div>
        <div className="msub">{t("mechanics.judge.sub")}</div>
        <div className="md md-jj">
          <div className="jj-card">
            <div className="jj-cv" style={{ background: "linear-gradient(140deg,#b91c1c,#f97316)" }} />
            <div className="jj-meta">
              <b>{d("jjTitle")}</b>
              <span>{d("jjReport")}</span>
            </div>
          </div>
          <div className="jj-side">
            <div className="jj-q">{d("jjVoting")}</div>
            <div className="jj-row">
              <span>{d("jjNo")}</span>
              <div className="jj-bar">
                <i className="b1" />
              </div>
              <b>3</b>
            </div>
            <div className="jj-row">
              <span>{d("jjYes")}</span>
              <div className="jj-bar">
                <i className="b2" />
              </div>
              <b>9</b>
            </div>
            <div className="jj-v">{d("jjVerdict")}</div>
          </div>
        </div>
      </div>

      {/* C 收藏集与系列（收进合集 → 追更） */}
      <div className="mcard">
        <div className="mh">
          <span className="mtag">{t("mechanics.collections.tag")}</span>
          <h3>{t("mechanics.collections.title")}</h3>
        </div>
        <div className="msub">{t("mechanics.collections.sub")}</div>
        <div className="md md-cl">
          <div className="cl-pool">
            <div className="cl-t">{d("clPool")}</div>
            <div className="cl-chip c1">
              <i style={{ background: "linear-gradient(140deg,#065f46,#34d399)" }} />
              <span>{d("clChip1")}</span>
            </div>
            <div className="cl-chip c2">
              <i style={{ background: "linear-gradient(140deg,#9d174d,#f472b6)" }} />
              <span>{d("clChip2")}</span>
            </div>
            <div className="cl-chip c3">
              <i style={{ background: "linear-gradient(140deg,#312e81,#818cf8)" }} />
              <span>{d("clChip3")}</span>
            </div>
          </div>
          <div className="cl-box">
            <div className="cl-h">
              {d("clBox")}
              <span className="cl-n">
                <b className="cn1">{d("clCount1")}</b>
                <b className="cn2">{d("clCount2")}</b>
                <b className="cn3">{d("clCount3")}</b>
              </span>
            </div>
            <div className="cl-slots">
              <i className="s1" />
              <i className="s2" />
              <i className="s3" />
            </div>
          </div>
        </div>
      </div>

      {/* D 全品类创作（品类轮换发布） */}
      <div className="mcard">
        <div className="mh">
          <span className="mtag">{t("mechanics.allMedia.tag")}</span>
          <h3>{t("mechanics.allMedia.title")}</h3>
        </div>
        <div className="msub">{t("mechanics.allMedia.sub")}</div>
        <div className="md md-me">
          <div className="me-tabs">
            <span className="t1">{d("meTab1")}</span>
            <span className="t2">{d("meTab2")}</span>
            <span className="t3">{d("meTab3")}</span>
            <span className="t4">{d("meTab4")}</span>
          </div>
          <div className="me-st">
            <div className="me-c c1">
              <i style={{ background: "linear-gradient(140deg,#9d174d,#f472b6)" }} />
              <div>
                <b>{d("meCard1t")}</b>
                <span>{d("meCard1m")}</span>
              </div>
            </div>
            <div className="me-c c2">
              <i style={{ background: "linear-gradient(140deg,#1e293b,#64748b)" }} />
              <div>
                <b>{d("meCard2t")}</b>
                <span>{d("meCard2m")}</span>
              </div>
            </div>
            <div className="me-c c3">
              <i style={{ background: "linear-gradient(140deg,#0f766e,#14b8a6)" }} />
              <div>
                <b>{d("meCard3t")}</b>
                <span>{d("meCard3m")}</span>
              </div>
            </div>
            <div className="me-c c4">
              <i style={{ background: "linear-gradient(140deg,#312e81,#818cf8)" }} />
              <div>
                <b>{d("meCard4t")}</b>
                <span>{d("meCard4m")}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
