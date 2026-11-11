import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  reactStrictMode: true,
  poweredByHeader: false,
  transpilePackages: ['@nn/tokens'],
  images: { unoptimized: true },
  experimental: { outputFileTracingRoot: path.join(here, '../../') },
};

export default nextConfig;
