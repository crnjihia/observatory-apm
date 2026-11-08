// path: C:/Users/crnji/Videos/Projects/Web Development/New/observatory-apm/web/next.config.js
/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  swcMinify: true,
  output: 'export',
  trailingSlash: true,
  basePath: process.env.GITHUB_PAGES === 'true' ? '/observatory-apm' : '',
};
module.exports = nextConfig;
