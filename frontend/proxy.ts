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

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;

  // Determine response — start with next(), then optionally redirect
  let response = NextResponse.next();

  // Set locale cookie from Accept-Language header if not already set
  if (!request.cookies.get('NEXT_LOCALE')) {
    const acceptLang = request.headers.get('accept-language') || '';
    const prefersZh = acceptLang.includes('zh');
    response.cookies.set('NEXT_LOCALE', prefersZh ? 'zh' : 'en', { path: '/' });
  }

  // Check auth for protected paths (always, even on first request)
  const isProtected = PROTECTED_PATHS.some(
    (p) => pathname === p || pathname.startsWith(p + '/'),
  );

  if (isProtected) {
    const token = request.cookies.get('access_token')?.value;
    if (!token) {
      const loginUrl = new URL('/login', request.url);
      loginUrl.searchParams.set('redirect', pathname);

      const redirect = NextResponse.redirect(loginUrl);
      // Carry over the locale cookie if we just set it
      const localeCookie = response.cookies.get('NEXT_LOCALE');
      if (localeCookie) {
        redirect.cookies.set('NEXT_LOCALE', localeCookie.value, localeCookie);
      }
      return redirect;
    }
  }

  return response;
}

export const config = {
  matcher: ['/((?!_next|api|favicon).*)'],
};
