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

  // Set locale cookie from Accept-Language header if not already set
  if (!request.cookies.get('NEXT_LOCALE')) {
    const acceptLang = request.headers.get('accept-language') || '';
    const prefersZh = acceptLang.includes('zh');
    const response = NextResponse.next();
    response.cookies.set('NEXT_LOCALE', prefersZh ? 'zh' : 'en', { path: '/' });
    return response;
  }

  const isProtected = PROTECTED_PATHS.some(
    (p) => pathname === p || pathname.startsWith(p + '/'),
  );

  if (!isProtected) return NextResponse.next();

  const token = request.cookies.get('access_token')?.value;
  if (!token) {
    const loginUrl = new URL('/login', request.url);
    loginUrl.searchParams.set('redirect', pathname);
    return NextResponse.redirect(loginUrl);
  }

  return NextResponse.next();
}

export const config = {
  matcher: ['/((?!_next|api|favicon).*)'],
};
