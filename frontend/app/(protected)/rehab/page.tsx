"use client";

import { useEffect, useState, useCallback, useRef, Fragment } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { BookOpen, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DataList } from "@/components/ui/data-list";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/Toast";
import { CourseCard } from "@/components/rehab/CourseCard";
import { CourseContent } from "@/components/rehab/CourseContent";
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
  const { toast } = useToast();
  const { user } = useAuth();
  const [courses, setCourses] = useState<Course[]>([]);
  const [completions, setCompletions] = useState<Completion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [activeId, setActiveId] = useState<number | null>(null);
  const [startTime, setStartTime] = useState<number | null>(null);
  const [elapsed, setElapsed] = useState(0);
  const [completingId, setCompletingId] = useState<number | null>(null);
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const loadData = useCallback(async (nextPage = 1, append = false) => {
    if (append) setLoadingMore(true); else setLoading(true);
    setError("");
    setPage(nextPage);
    try {
      const [coursesRes, progressRes] = await Promise.all([
        api.get<{ courses?: Course[]; total?: number; page_size?: number }>(`/api/v1/rehab/courses?page=${nextPage}&page_size=20`),
        api.get<{ completions?: Completion[] }>("/api/v1/rehab/my-progress"),
      ]);
      const incoming = coursesRes.courses ?? [];
      setCourses((current) => append ? [...current, ...incoming.filter((item) => !current.some((existing) => existing.id === item.id))] : incoming);
      setPage(nextPage);
      setHasMore((coursesRes.total ?? incoming.length) > nextPage * (coursesRes.page_size ?? 20));
      setCompletions(progressRes.completions ?? []);
    } catch (e) {
      silentError(e, { component: 'RehabPage', action: 'loadData' });
      const message = t(getUserFacingErrorKey(e, "common.loadFailed"));
      setError(message);
      toast("error", message);
    } finally {
      setLoadingMore(false);
      setLoading(false);
    }
  }, [t, toast]);

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
      setError(t(getUserFacingErrorKey(e)));
    } finally {
      setCompletingId(null);
    }
  }

  function isCompleted(courseId: number) {
    return completions.some((c) => c.course_id === courseId && c.completed_at);
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6 px-4 py-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("rehab.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("rehab.subtitle")}</p>
      </div>

      <DataList
        items={courses}
        loading={loading}
        error={error}
        onRetry={() => void loadData(page, page > 1)}
        hasMore={hasMore}
        loadingMore={loadingMore}
        onLoadMore={() => loadData(page + 1, true)}
        empty={<EmptyState icon={BookOpen} title={t("rehab.noCourses")} />}
        loadingState={<div className="space-y-3"><Skeleton className="h-28 w-full" /><Skeleton className="h-28 w-full" /><Skeleton className="h-28 w-full" /></div>}
        getKey={(course) => course.id}
        renderItem={(course) => {
            const done = isCompleted(course.id);
            const isActive = activeId === course.id;
            const canComplete = isActive && elapsed >= course.min_reading_sec;

            return (
              <Fragment key={course.id}>
              <CourseCard
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
              {isActive && course.content_i18n && (
                <div className="mt-2 rounded-md border border-border bg-card/50 p-4">
                  <CourseContent contentI18n={course.content_i18n} violationType={course.violation_type} />
                </div>
              )}
              </Fragment>
            );
        }}
      />
    </div>
  );
}
