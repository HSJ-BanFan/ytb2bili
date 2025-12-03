/** @type {import('next').NextConfig} */
const nextConfig = {
  env: {
    BACKEND_URL: process.env.BACKEND_URL || 'http://localhost:8096',
  },
  
  // 静态导出配置 - 用于生产构建
  output: 'export',
  trailingSlash: true,
  images: {
    unoptimized: true
  },
  distDir: 'out',
  
  // 基础路径配置（如果需要部署到子路径）
  // basePath: '/static',
  // assetPrefix: '/static',

  // 图片域名白名单配置
  images: {
    unoptimized: true, // 静态导出需要关闭图片优化
    remotePatterns: [
      {
        protocol: 'https',
        hostname: 'passport.bilibili.com',
        pathname: '/x/passport-tv-login/h5/qrcode/auth/**',
      },
    ],
  },
}

module.exports = nextConfig