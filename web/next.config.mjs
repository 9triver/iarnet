/** @type {import('next').NextConfig} */
const nextConfig = {
  eslint: {
    ignoreDuringBuilds: true,
  },
  typescript: {
    ignoreBuildErrors: true,
  },
  images: {
    unoptimized: true,
  },
  async rewrites() {
    const backendUrl = process.env.BACKEND_URL || 'http://localhost:8083'
    return [
      {
        source: '/api/:path*',
        destination: `${backendUrl}/:path*`
      }
    ]
  }
}

export default nextConfig
