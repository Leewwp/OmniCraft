"use client";

import { useEffect, useState, useCallback, useMemo } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { useRouter } from "next/navigation";
import ExamQuestion from "@/components/judge/ExamQuestion";

interface ExamQuestionData {
  id: number;
  question: {
    prompt: string;
    options: Record<string, string>;
  };
}

type Phase = "select-type" | "exam" | "result";

export default function JudgeExamPage() {
  const t = useTranslations();
  const { user } = useAuth();
  const router = useRouter();

  const [phase, setPhase] = useState<Phase>("select-type");
  const [contentType, setContentType] = useState("");
  const [questions, setQuestions] = useState<ExamQuestionData[]>([]);
  const [currentIndex, setCurrentIndex] = useState(0);
  const [answers, setAnswers] = useState<Record<number, string>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<{ passed: boolean; score: number; total: number } | null>(null);

  const CONTENT_TYPES = useMemo(() => [
    { value: "article", label: t('judge.article') },
    { value: "image", label: t('judge.image') },
    { value: "video", label: t('judge.video') },
    { value: "audio", label: t('judge.audio') },
    { value: "prompt", label: t('judge.prompt') },
    { value: "comment", label: t('judge.comment') },
    { value: "sheet_music", label: t('judge.sheetMusic') },
    { value: "template", label: t('judge.template') },
    { value: "other", label: t('judge.other') },
  ], [t]);

  async function loadQuestions(category: string) {
    setLoading(true);
    setError("");
    try {
      const data = await api.get<{ questions: ExamQuestionData[] }>(`/api/v1/judge/exam/${category}`);
      if (!data.questions || data.questions.length === 0) {
        setError(t('judge.examNoQuestions'));
        return;
      }
      setQuestions(data.questions);
      setPhase("exam");
      setCurrentIndex(0);
      setAnswers({});
    } catch (e) {
      silentError(e, { component: 'JudgeExamPage', action: 'loadQuestions' });
      setError(t(getUserFacingErrorKey(e, "judge.examLoadFailed")));
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
      silentError(e, { component: 'JudgeExamPage', action: 'submitExam' });
      setError(t(getUserFacingErrorKey(e, "judge.examSubmitFailed")));
    } finally {
      setLoading(false);
    }
  }

  const isReputationBlocked = user!.reputation < 3;

  return (
    <div className="mx-auto w-full max-w-lg space-y-4 px-4 py-6">
      <div className="rounded-md border border-border bg-card p-4 ">
        <h1 className="text-2xl font-bold tracking-tight">{t('judge.examTitle')}</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {t('judge.examSubtitle')}
        </p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {isReputationBlocked && (
        <div className="rounded-md border border-destructive/50 bg-destructive/5 p-4 ">
          <p className="text-sm text-destructive">
            {t('judge.lowReputationExam', { reputation: user!.reputation })}
          </p>
        </div>
      )}

      {phase === "select-type" && (
        <div className="space-y-3 rounded-md border border-border bg-card p-4 ">
          <p className="text-sm font-medium">{t('judge.examSelectType')}</p>
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
          <div className="rounded-md border border-border bg-card p-3 ">
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span>
                {t('judge.examQuestion', { current: currentIndex + 1, total: questions.length })}
              </span>
              <span>
                {t('judge.examUnanswered', { count: questions.length - Object.keys(answers).length })}
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
              {t('judge.examPrev')}
            </Button>
            {currentIndex < questions.length - 1 ? (
              <Button size="sm" onClick={goNext}>
                {t('judge.examNext')}
              </Button>
            ) : (
              <Button
                size="sm"
                disabled={Object.keys(answers).length < questions.length || loading}
                onClick={() => void submitExam()}
              >
                {loading ? t('judge.examSubmitting') : t('judge.examSubmit')}
              </Button>
            )}
          </div>
        </>
      )}

      {phase === "result" && result && (
        <div className="space-y-4 rounded-md border border-border bg-card p-6 text-center ">
          <div className="text-3xl font-bold tracking-tight">
            {result.passed ? (
              <span className="text-emerald-600">{t('judge.examPassed')}</span>
            ) : (
              <span className="text-destructive">{t('judge.examFailed')}</span>
            )}
          </div>
          <p className="text-lg text-muted-foreground">
            {t('judge.examScore', { score: result.score, total: result.total })}
            {t('judge.examAccuracy', { accuracy: Math.round((result.score / result.total) * 100) })}
          </p>
          <p className="text-sm text-muted-foreground">
            {result.passed
              ? t('judge.examPassedMsg')
              : t('judge.examFailedMsg')}
          </p>
          <div className="flex justify-center gap-2">
            {!result.passed && (
              <Button variant="outline" size="sm" onClick={() => setPhase("select-type")}>
                {t('judge.examRetry')}
              </Button>
            )}
            <Button size="sm" onClick={() => router.push("/judge/queue")}>
              {t('judge.examGoToQueue')}
            </Button>
          </div>
        </div>
      )}

      {phase === "select-type" && loading && (
        <div className="flex items-center justify-center rounded-md border border-border bg-card p-8 ">
          <span className="text-sm text-muted-foreground">{t('judge.examLoading')}</span>
        </div>
      )}
    </div>
  );
}
