import type { Metadata } from 'next';
import { NextIntlClientProvider } from 'next-intl';
import { getLocale, getMessages, getTranslations } from 'next-intl/server';
import { ThemeProvider } from 'next-themes';
import { AuthProvider } from '@/contexts/AuthContext';
import { ToastProvider } from '@/components/ui/Toast';
import { AgentChatWidget } from '@/components/agent/AgentChatWidget';
import { AgentFeatureGate } from '@/components/agent/AgentFeatureGate';
import './globals.css';

export const metadata: Metadata = {
  title: 'OmniCraft',
  description: 'Creative sharing platform for everyone',
};

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const locale = await getLocale();
  const messages = await getMessages();
  const t = await getTranslations('common');

  return (
    <html
      lang={locale === 'en' ? 'en-US' : 'zh-CN'}
      suppressHydrationWarning
      className="h-full antialiased"
    >
      <body className="min-h-full flex flex-col bg-background text-foreground">
        <a
          href="#main-content"
          className="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50 focus:rounded-md focus:bg-accent-emphasis focus:px-4 focus:py-2 focus:text-sm focus:text-white focus:outline-none"
        >
          {t('skipToContent')}
        </a>
        <NextIntlClientProvider messages={messages} locale={locale}>
          <ThemeProvider
            attribute="class"
            defaultTheme="system"
            enableSystem
            disableTransitionOnChange
          >
            <ToastProvider>
              <AuthProvider>
                <main id="main-content" className="flex-1">
                  {children}
                </main>
                <AgentFeatureGate capability="webAgent">
                  <AgentChatWidget />
                </AgentFeatureGate>
              </AuthProvider>
            </ToastProvider>
          </ThemeProvider>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
