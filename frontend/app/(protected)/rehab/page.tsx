"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { BookOpen, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { CourseCard } from "@/components/rehab/CourseCard";
import { ReputationDetail } from "@/components/rehab/ReputationDetail";

interface Course {
  id: number;
  violation_type: string;
  content_i18n?: Record<string, string>;
  min_reading_sec: number;
  reward_points: number;
}

interface Completion {
  course_id: number;
  started_at?: string;
  completed_at?: string;
}

export default function RehabPage() {
  const t = useTranslations();
  const { user } = useAuth();
  const [courses, setCourses] = useState<Course[]>([]);
  const [completions, setCompletions] = useState<Completion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [activeId, setActiveId] = useState<number | null>(null);
  const [startTime, setStartTime] = useState<number | null>(null);
  const [elapsed, setElapsed] = useState(0);
  const [completingId, setCompletingId] = useState<number | null>(null);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const loadData = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [coursesRes, progressRes] = await Promise.all([
        api.get<{ courses?: Course[] }>("/api/v1/rehab/courses"),
        api.get<{ completions?: Completion[] }>("/api/v1/rehab/my-progress"),
      ]);
      setCourses(coursesRes.courses ?? []);
      setCompletions(progressRes.completions ?? []);
    } catch (e) {
      silentError(e, { component: 'RehabPage', action: 'loadData' });
      setError(e instanceof ApiRequestError ? e.message : t("common.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    if (!user) return;
    loadData();
  }, [user, loadData]);

  async function handleStart(courseId: number, minSec: number) {
    setActiveId(courseId);
    setStartTime(Date.now());
    setElapsed(0);
    try {
      await api.post(`/api/v1/rehab/courses/${courseId}/start`, {});
      const start = Date.now();
      timerRef.current = setInterval(() => {
        const e = Math.floor((Date.now() - start) / 1000);
        setElapsed(e);
      }, 1000);
    } catch (e) {
      silentError(e, { component: 'RehabPage', action: 'handleStart' });
      setActiveId(null);
      setStartTime(null);
    }
  }

  async function handleComplete(courseId: number) {
    if (timerRef.current) clearInterval(timerRef.current);
    setCompletingId(courseId);
    try {
      await api.post(`/api/v1/rehab/courses/${courseId}/complete`, {});
      setActiveId(null);
      setStartTime(null);
      await loadData();
    } catch (e) {
      silentError(e, { component: 'RehabPage', action: 'handleComplete' });
      setError(e instanceof ApiRequestError ? e.message : t("common.operationFailed"));
    } finally {
      setCompletingId(null);
    }
  }

  function isCompleted(courseId: number) {
    return completions.some((c) => c.course_id === courseId && c.completed_at);
  }

  if (loading) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-6 text-sm text-muted-foreground">
        {t("common.loading")}
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6 px-4 py-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("rehab.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("rehab.subtitle")}</p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {courses.length === 0 ? (
        <div className="flex flex-col items-center gap-3 rounded-md border border-border bg-card p-8 text-center">
          <BookOpen className="h-8 w-8 text-muted-foreground/40" />
          <p className="text-sm text-muted-foreground">{t("rehab.noCourses")}</p>
        </div>
      ) : (
        <div className="space-y-3">
          {courses.map((course) => {
            const done = isCompleted(course.id);
            const isActive = activeId === course.id;
            const canComplete = isActive && elapsed >= course.min_reading_sec;

            return (
              <CourseCard
                key={course.id}
                violationType={course.violation_type}
                contentI18n={course.content_i18n}
                minReadingSec={course.min_reading_sec}
                rewardPoints={course.reward_points}
                isActive={isActive}
                elapsed={elapsed}
              >
                {done ? (
                  <ReputationDetail rewardPoints={course.reward_points} completed />
                ) : isActive ? (
                  <Button
                    size="sm"
                    onClick={() => handleComplete(course.id)}
                    disabled={!canComplete || completingId === course.id}
                  >
                    {completingId === course.id ? (
                      <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" />
                    ) : null}
                    {canComplete ? t("rehab.complete") : t("rehab.pleaseRead")}
                  </Button>
                ) : (
                  <Button size="sm" onClick={() => handleStart(course.id, course.min_reading_sec)}>
                    {t("rehab.startLearning")}
                  </Button>
                )}
              </CourseCard>
            );
          })}
        </div>
      )}
    </div>
  );
}
