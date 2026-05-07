import createMiddleware from 'next-intl/middleware';
import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';
import { routing } from './i18n/routing';

const intlMiddleware = createMiddleware(routing);

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

export default function middleware(request: NextRequest) {
  const intlResponse = intlMiddleware(request);

  // If i18n middleware redirected (locale detection), return immediately
  if (intlResponse.status >= 300 && intlResponse.status < 400) {
    return intlResponse;
  }

  const { pathname } = request.nextUrl;

  const isProtected = PROTECTED_PATHS.some(
    (p) => pathname === p || pathname.startsWith(p + '/')
  );

  if (!isProtected) return intlResponse;

  const token = request.cookies.get('access_token')?.value;
  if (!token) {
    const loginUrl = new URL('/login', request.url);
    loginUrl.searchParams.set('redirect', pathname);

    const redirect = NextResponse.redirect(loginUrl);
    // Preserve locale cookie if set by i18n middleware
    const localeCookie = intlResponse.cookies.get('NEXT_LOCALE');
    if (localeCookie) {
      redirect.cookies.set('NEXT_LOCALE', localeCookie.value, localeCookie);
    }
    return redirect;
  }

  return intlResponse;
}

export const config = {
  matcher: ['/((?!_next/static|_next/image|favicon.ico|api/).*)'],
};
