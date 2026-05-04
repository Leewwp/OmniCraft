"use client";

import { useEffect, useState, useCallback } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { useRouter } from "next/navigation";
import ExamQuestion from "@/components/judge/ExamQuestion";

const CONTENT_TYPES = [
  { value: "article", label: "文章" },
  { value: "image", label: "图片" },
  { value: "video", label: "视频" },
  { value: "audio", label: "音频" },
  { value: "prompt", label: "提示词" },
  { value: "comment", label: "评论" },
  { value: "sheet_music", label: "乐谱" },
  { value: "template", label: "模板" },
  { value: "other", label: "其他" },
];

interface ExamQuestionData {
  id: number;
  question: {
    prompt: string;
    options: Record<string, string>;
  };
}

type Phase = "select-type" | "exam" | "result";

export default function JudgeExamPage() {
  const { user, isLoading: authLoading } = useAuth();
  const router = useRouter();

  const [phase, setPhase] = useState<Phase>("select-type");
  const [contentType, setContentType] = useState("");
  const [questions, setQuestions] = useState<ExamQuestionData[]>([]);
  const [currentIndex, setCurrentIndex] = useState(0);
  const [answers, setAnswers] = useState<Record<number, string>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<{ passed: boolean; score: number; total: number } | null>(null);

  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/login");
    }
  }, [user, authLoading, router]);

  async function loadQuestions(category: string) {
    setLoading(true);
    setError("");
    try {
      const data = await api.get<{ questions: ExamQuestionData[] }>(`/api/v1/judge/exam/${category}`);
      if (!data.questions || data.questions.length === 0) {
        setError("该类型暂无可用的考题");
        return;
      }
      setQuestions(data.questions);
      setPhase("exam");
      setCurrentIndex(0);
      setAnswers({});
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "加载考题失败");
    } finally {
      setLoading(false);
    }
  }

  function handleSelect(key: string) {
    if (!questions[currentIndex]) return;
    setAnswers((prev) => ({ ...prev, [questions[currentIndex].id]: key }));
  }

  function goNext() {
    if (currentIndex < questions.length - 1) {
      setCurrentIndex((i) => i + 1);
    }
  }

  function goPrev() {
    if (currentIndex > 0) {
      setCurrentIndex((i) => i - 1);
    }
  }

  async function submitExam() {
    setLoading(true);
    setError("");
    try {
      const payload = {
        content_type: contentType,
        answers: Object.entries(answers).map(([qid, key]) => ({
          question_id: parseInt(qid),
          answer_key: key,
        })),
      };
      const data = await api.post<{ record: { score: number; total: number; passed: boolean }; passed: boolean }>(
        "/api/v1/judge/exam/submit",
        payload
      );
      setResult({ passed: data.passed, score: data.record.score, total: data.record.total });
      setPhase("result");
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "提交失败");
    } finally {
      setLoading(false);
    }
  }

  if (authLoading) {
    return (
      <div className="mx-auto w-full max-w-lg px-4 py-6 text-sm text-muted-foreground">
        加载中...
      </div>
    );
  }

  if (!user) return null;

  const isReputationBlocked = user.reputation < 3;

  return (
    <div className="mx-auto w-full max-w-lg space-y-4 px-4 py-6">
      <div className="rounded-md border border-border bg-card p-4 shadow-none">
        <h1 className="text-2xl font-bold tracking-tight">赛博判官资质考核</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          通过考核后即可参与对应类型内容的众裁审核
        </p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {isReputationBlocked && (
        <div className="rounded-md border border-destructive/50 bg-destructive/5 p-4 shadow-none">
          <p className="text-sm text-destructive">
            信誉分不足（当前 {user.reputation} 分，需要 ≥ 3 分），无法参加考核。请通过素质建设课程恢复信誉分。
          </p>
        </div>
      )}

      {phase === "select-type" && (
        <div className="space-y-3 rounded-md border border-border bg-card p-4 shadow-none">
          <p className="text-sm font-medium">选择考核内容类型</p>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
            {CONTENT_TYPES.map((ct) => (
              <button
                key={ct.value}
                type="button"
                disabled={isReputationBlocked || loading}
                onClick={() => {
                  setContentType(ct.value);
                  void loadQuestions(ct.value);
                }}
                className={`rounded-md border px-4 py-3 text-sm transition-all focus-visible:ring-2 focus-visible:ring-accent ${
                  isReputationBlocked || loading
                    ? "border-border bg-background opacity-50 cursor-not-allowed"
                    : "border-border bg-background text-foreground hover:bg-muted cursor-pointer"
                }`}
              >
                {ct.label}
              </button>
            ))}
          </div>
        </div>
      )}

      {phase === "exam" && questions.length > 0 && (
        <>
          <div className="rounded-md border border-border bg-card p-3 shadow-none">
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span>
                第 {currentIndex + 1} / {questions.length} 题
              </span>
              <span>
                {questions.length - Object.keys(answers).length} 题未答
              </span>
            </div>
            <div className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-muted">
              <div
                className="h-full rounded-full bg-accent transition-all duration-300"
                style={{ width: `${((currentIndex + 1) / questions.length) * 100}%` }}
              />
            </div>
          </div>

          <ExamQuestion
            question={questions[currentIndex]}
            selectedKey={answers[questions[currentIndex].id] || ""}
            onSelect={handleSelect}
            disabled={loading}
          />

          <div className="flex items-center justify-between gap-2">
            <Button variant="outline" size="sm" disabled={currentIndex === 0 || loading} onClick={goPrev}>
              上一题
            </Button>
            {currentIndex < questions.length - 1 ? (
              <Button size="sm" onClick={goNext}>
                下一题
              </Button>
            ) : (
              <Button
                size="sm"
                disabled={Object.keys(answers).length < questions.length || loading}
                onClick={() => void submitExam()}
              >
                {loading ? "提交中..." : "提交答案"}
              </Button>
            )}
          </div>
        </>
      )}

      {phase === "result" && result && (
        <div className="space-y-4 rounded-md border border-border bg-card p-6 text-center shadow-none">
          <div className="text-3xl font-bold tracking-tight">
            {result.passed ? (
              <span className="text-emerald-600">考核通过</span>
            ) : (
              <span className="text-destructive">未通过</span>
            )}
          </div>
          <p className="text-lg text-muted-foreground">
            得分：{result.score} / {result.total}
            （正确率 {Math.round((result.score / result.total) * 100)}%）
          </p>
          <p className="text-sm text-muted-foreground">
            {result.passed
              ? "你已获得该类型的判官资格，可前往审核队列参与众裁。"
              : "正确率需达到 80% 以上，可重新选择类型再次考核。"}
          </p>
          <div className="flex justify-center gap-2">
            {!result.passed && (
              <Button variant="outline" size="sm" onClick={() => setPhase("select-type")}>
                重新选择
              </Button>
            )}
            <Button size="sm" onClick={() => router.push("/judge/queue")}>
              前往审核队列
            </Button>
          </div>
        </div>
      )}

      {phase === "select-type" && loading && (
        <div className="flex items-center justify-center rounded-md border border-border bg-card p-8 shadow-none">
          <span className="text-sm text-muted-foreground">加载考题中...</span>
        </div>
      )}
    </div>
  );
}
