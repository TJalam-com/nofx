import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    // Optimize chunk splitting for better caching
    rollupOptions: {
      output: {
        manualChunks: (id) => {
          // Vendor chunks - separate large libraries for better caching
          if (id.includes('node_modules')) {
            // React core libraries
            if (id.includes('react') || id.includes('react-dom') || id.includes('react-router')) {
              return 'react-vendor'
            }
            // Large animation library - separate chunk
            if (id.includes('framer-motion')) {
              return 'framer-motion'
            }
            // Large charting library - separate chunk (only loaded when needed)
            if (id.includes('recharts')) {
              return 'recharts'
            }
            // Icon library - separate chunk
            if (id.includes('lucide-react')) {
              return 'lucide-react'
            }
            // UI component library
            if (id.includes('@radix-ui')) {
              return 'radix-ui'
            }
            // Data fetching library
            if (id.includes('swr')) {
              return 'swr'
            }
            // HTTP client
            if (id.includes('axios')) {
              return 'axios'
            }
            // State management
            if (id.includes('zustand')) {
              return 'zustand'
            }
            // Other vendor libraries
            return 'vendor'
          }
        },
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
        target: 'http://localhost:8080',
        changeOrigin: true,
        configure: (proxy, _options) => {
          proxy.on('error', (err, _req, _res) => {
            if (err.code === 'ECONNREFUSED') {
              console.error('\n⚠️  Backend connection error: Backend server is not running!')
              console.error('   Please start the backend server with: ./nofx')
              console.error('   Or: go run main.go\n')
            } else {
              console.error('Backend proxy error:', err.message)
            }
          })
        },
      },
    },
  },
})
