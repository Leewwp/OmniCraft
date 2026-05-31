export interface AgentPageContext {
  route: string;
  contentId?: number;
  contentTitle?: string;
  contentType?: string;
}

const ALLOWED_ROUTES_PREFIXES = [
  "/contents/",
  "/original/",
  "/fanwork/",
  "/studio/",
  "/search",
];

export function buildPageContext(): AgentPageContext {
  const pathname = typeof window !== "undefined" ? window.location.pathname : "";

  const ctx: AgentPageContext = {
    route: pathname,
  };

  const contentMatch = pathname.match(/\/(?:contents|original|fanwork)\/(\d+)/);
  if (contentMatch) {
    ctx.contentId = parseInt(contentMatch[1], 10);
  }

  return ctx;
}

export function sanitizePageContext(ctx: AgentPageContext): AgentPageContext {
  const sanitized: AgentPageContext = {
    route: typeof ctx.route === "string" ? ctx.route.slice(0, 200) : "",
  };

  if (typeof ctx.contentId === "number" && Number.isFinite(ctx.contentId) && ctx.contentId > 0) {
    sanitized.contentId = ctx.contentId;
  }

  if (typeof ctx.contentTitle === "string" && ctx.contentTitle.length <= 200) {
    sanitized.contentTitle = ctx.contentTitle;
  }

  if (typeof ctx.contentType === "string" && ctx.contentType.length <= 50) {
    sanitized.contentType = ctx.contentType;
  }

  return sanitized;
}

export const QUICK_PROMPTS = [
  "page_help",
  "download_help",
  "publish_help",
  "desktop_client_help",
  "report_problem",
] as const;

export type QuickPromptIntent = (typeof QUICK_PROMPTS)[number];
