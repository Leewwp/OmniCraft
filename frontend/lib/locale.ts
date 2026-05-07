import { api } from './api';

const LOCALE_COOKIE = 'NEXT_LOCALE';
const LOCALE_STORAGE = 'omnicraft_locale';

const SUPPORTED_LOCALES = ['zh', 'en'] as const;
type Locale = (typeof SUPPORTED_LOCALES)[number];

function getBrowserLocale(): Locale {
  if (typeof navigator === 'undefined') return 'zh';
  const lang = navigator.language;
  if (lang.startsWith('zh')) return 'zh';
  if (lang.startsWith('en')) return 'en';
  return 'zh';
}

function readCookie(name: string): string | null {
  if (typeof document === 'undefined') return null;
  const match = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`));
  return match ? decodeURIComponent(match[1]) : null;
}

function writeCookie(name: string, value: string) {
  if (typeof document === 'undefined') return;
  document.cookie = `${name}=${encodeURIComponent(value)}; path=/; max-age=31536000; SameSite=Lax`;
}

export function getLocale(): Locale {
  const cookieLocale = readCookie(LOCALE_COOKIE);
  if (cookieLocale && SUPPORTED_LOCALES.includes(cookieLocale as Locale)) {
    return cookieLocale as Locale;
  }

  if (typeof localStorage !== 'undefined') {
    const stored = localStorage.getItem(LOCALE_STORAGE);
    if (stored && SUPPORTED_LOCALES.includes(stored as Locale)) {
      return stored as Locale;
    }
  }

  return getBrowserLocale();
}

export async function setLocale(locale: Locale, userId?: string | number): Promise<void> {
  if (!SUPPORTED_LOCALES.includes(locale)) return;

  writeCookie(LOCALE_COOKIE, locale);

  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(LOCALE_STORAGE, locale);
  }

  if (userId) {
    try {
      await api.patch(`/api/v1/users/${userId}`, { preferred_locale: locale === 'en' ? 'en-US' : 'zh-CN' });
    } catch {
      // Silently ignore server sync failures — locale is persisted client-side
    }
  }
}
