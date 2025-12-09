import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    // Let Vite handle chunk splitting automatically to avoid circular dependencies
    rollupOptions: {
      output: {
        // Optimize chunk file names
        chunkFileNames: 'assets/[name]-[hash].js',
        entryFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash].[ext]',
      },
    },
    // Enable CSS code splitting
    cssCodeSplit: true,
    // Optimize asset handling
    assetsInlineLimit: 4096, // Inline assets smaller than 4kb
    // Minification (enabled by default in production)
    minify: 'esbuild', // Use esbuild for faster minification
    // Source maps for production debugging (optional, increases build time)
    sourcemap: false,
    // Target modern browsers for smaller bundles
    target: 'esnext',
    // Chunk size warning limit (1MB)
    chunkSizeWarningLimit: 1000,
    // Report compressed size
    reportCompressedSize: true,
  },
  server: {
    host: '0.0.0.0',
    port: 3000,
    proxy: {
      '/api': {
        target: process.env.VITE_API_URL || 'http://localhost:8080',
        changeOrigin: true,
        configure: (proxy, _options) => {
          proxy.on('error', (err, _req, _res) => {
            const nodeError = err as NodeJS.ErrnoException
            if (nodeError.code === 'ECONNREFUSED') {
              console.error('\n⚠️  Backend connection error: Backend server is not running!')
              console.error('   Please start the backend server with: ./nofx')
              console.error('   Or: go run main.go\n')
            } else {
              console.error('Backend proxy error:', nodeError.message)
            }
          })
        },
      },
    },
  },
})
