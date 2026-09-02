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
  // 旧 IP 子路由 301 收敛到单页 query 形态（#290）：类目页 → ?tab=share&type=，
  // 讨论区列表 → ?tab=discussions、帖详情 → ?tab=discussions&d=<id>。
  // 注意：具体路由必须排在泛型 :category 之前（Next 按数组顺序取首条命中），
  // 且 :discussionId 用数字约束，避免吞掉 /discussions/new 发帖页路由
  // （next.config redirects 先于文件系统路由执行）。
  async redirects() {
    return [
      {
        source: '/ip/:ipId/discussions',
        destination: '/ip/:ipId?tab=discussions',
        permanent: true,
      },
      {
        source: '/ip/:ipId/discussions/:discussionId(\\d+)',
        destination: '/ip/:ipId?tab=discussions&d=:discussionId',
        permanent: true,
      },
      {
        source: '/ip/:ipId/:category',
        destination: '/ip/:ipId?tab=share&type=:category',
        permanent: true,
      },
    ];
  },
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
