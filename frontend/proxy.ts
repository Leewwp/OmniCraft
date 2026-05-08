import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

const PROTECTED_PATHS = [
  '/dashboard',
  '/judge',
  '/publish',
  '/settings',
  '/history',
  '/appeals',
  '/messages',
  '/rehab',
  '/admin',
];

function decodeJWTPayload(token: string): Record<string, unknown> | null {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return null;
    const payload = atob(parts[1]);
    return JSON.parse(payload);
  } catch {
    return null;
  }
}

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;

  let response = NextResponse.next();

  // Set locale cookie from Accept-Language header if not already set
  if (!request.cookies.get('NEXT_LOCALE')) {
    const acceptLang = request.headers.get('accept-language') || '';
    const prefersZh = acceptLang.includes('zh');
    response.cookies.set('NEXT_LOCALE', prefersZh ? 'zh' : 'en', { path: '/' });
  }

  const isProtected = PROTECTED_PATHS.some(
    (p) => pathname === p || pathname.startsWith(p + '/'),
  );

  if (!isProtected) return response;

  const token = request.cookies.get('access_token')?.value;
  if (!token) {
    const loginUrl = new URL('/login', request.url);
    loginUrl.searchParams.set('redirect', pathname);

    const redirect = NextResponse.redirect(loginUrl);
    const localeCookie = response.cookies.get('NEXT_LOCALE');
    if (localeCookie) {
      redirect.cookies.set('NEXT_LOCALE', localeCookie.value, localeCookie);
    }
    return redirect;
  }

  // Role check for admin routes
  if (pathname.startsWith('/admin')) {
    const payload = decodeJWTPayload(token);
    if (!payload || payload.role !== 'admin') {
      return NextResponse.redirect(new URL('/', request.url));
    }
  }

  return response;
}

export const config = {
  matcher: ['/((?!_next|api|favicon).*)'],
};
