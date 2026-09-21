/**
 * SP-25 FR-05（中-11）：登录 redirect 参数的白名单校验——只放行站内路径
 * （单个 "/" 开头且非协议相对 "//"），其余一律回退 "/"。防
 * login?redirect=//evil.com 把登录成功的用户送去仿真登录页二次钓密码。
 */
export function safeRedirectPath(value: string | null | undefined): string {
  if (!value) {
    return "/";
  }
  if (!value.startsWith("/") || value.startsWith("//")) {
    return "/";
  }
  return value;
}
