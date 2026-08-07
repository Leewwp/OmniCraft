import type { NextConfig } from 'next';
import createNextIntlPlugin from 'next-intl/plugin';

const withNextIntl = createNextIntlPlugin('./i18n/request.ts');

const nextConfig: NextConfig = {
  output: 'standalone',
  // Mocked Playwright runs alongside the developer server during the full
  // verification gate. Keep its build lock separate so the two servers can
  // coexist without changing the production `.next` output.
  distDir: process.env.NEXT_DIST_DIR || '.next',
  allowedDevOrigins: ['127.0.0.1'],
  images: {
    remotePatterns: [
      {
        protocol: 'https',
        hostname: '*.aliyuncs.com',
      },
      {
        protocol: 'https',
        hostname: '*.oss-cn-*.aliyuncs.com',
      },
    ],
  },
};

export default withNextIntl(nextConfig);
