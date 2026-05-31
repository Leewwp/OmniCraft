import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

const SUPPORTED_LOCALES = ['zh', 'en'];
const DEFAULT_LOCALE = 'zh';

const PROTECTED_PATHS = [
  '/dashboard',
  '/studio',
  '/judge',
  '/publish',
  '/settings',
  '/history',
  '/appeals',
  '/messages',
  '/rehab',
  '/admin',
];

function resolveLocale(request: NextRequest): string {
  const cookieLocale = request.cookies.get('NEXT_LOCALE')?.value;
  if (cookieLocale && SUPPORTED_LOCALES.includes(cookieLocale)) {
    return cookieLocale;
  }

  const acceptLang = request.headers.get('accept-language') || '';
  if (acceptLang.includes('zh')) return 'zh';
  if (acceptLang.includes('en')) return 'en';

  return DEFAULT_LOCALE;
}

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;

  const locale = resolveLocale(request);

  const requestHeaders = new Headers(request.headers);
  requestHeaders.set('X-NEXT-INTL-LOCALE', locale);

  let response = NextResponse.next({
    request: { headers: requestHeaders },
  });

  if (request.cookies.get('NEXT_LOCALE')?.value !== locale) {
    response.cookies.set('NEXT_LOCALE', locale, { path: '/', sameSite: 'lax' });
  }

  const isProtected = PROTECTED_PATHS.some(
    (p) => pathname === p || pathname.startsWith(p + '/'),
  );

  if (!isProtected) return response;

  return response;
}

export const config = {
  matcher: ['/((?!_next|api|favicon).*)'],
};
