"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { BookOpen, Check, Clock, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";

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
      // Start countdown
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
              <div key={course.id} className="rounded-md border border-border bg-card p-4 ">
                <div className="flex items-start justify-between gap-4">
                  <div className="min-w-0 flex-1">
                    <h3 className="text-sm font-semibold">
                      {course.content_i18n?.zh || course.violation_type}
                    </h3>
                    <div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                      <span className="inline-flex items-center gap-1">
                        <Clock className="h-3 w-3" />
                        {t("rehab.minReading", { sec: course.min_reading_sec })}
                      </span>
                      <span className="inline-flex items-center gap-1">
                        <BookOpen className="h-3 w-3" />
                        {t("rehab.rewardPoints", { pts: course.reward_points })}
                      </span>
                    </div>

                    {isActive && (
                      <div className="mt-3 space-y-1">
                        <div className="relative h-2 w-full rounded-full bg-muted/40">
                          <div
                            className="absolute inset-y-0 left-0 rounded-full bg-accent transition-[width] duration-1000"
                            style={{
                              width: `${Math.min((elapsed / course.min_reading_sec) * 100, 100)}%`,
                            }}
                          />
                        </div>
                        <p className="text-xs text-muted-foreground">
                          {t("rehab.readingProgress", { elapsed, required: course.min_reading_sec })}
                        </p>
                      </div>
                    )}
                  </div>

                  <div className="shrink-0">
                    {done ? (
                      <span className="inline-flex items-center gap-1 rounded bg-emerald-100 px-2 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400">
                        <Check className="h-3 w-3" />
                        {t("rehab.completed")}
                      </span>
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
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
