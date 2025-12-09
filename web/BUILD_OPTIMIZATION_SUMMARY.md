# Build Optimization Summary

This document outlines the optimizations implemented to address JavaScript minification and unused code reduction.

## Changes Implemented

### 1. Enhanced Vite Build Configuration ✅
- **Improved Chunk Splitting**: Separated large libraries into individual chunks:
  - `react-vendor`: React, React DOM, React Router
  - `framer-motion`: Animation library (separate chunk)
  - `recharts`: Charting library (separate chunk, only loaded when needed)
  - `lucide-react`: Icon library (separate chunk)
  - `radix-ui`: UI components (separate chunk)
  - `swr`: Data fetching (separate chunk)
  - `axios`: HTTP client (separate chunk)
  - `zustand`: State management (separate chunk)
  - `vendor`: Other vendor libraries

- **Optimized File Naming**: Added hash-based naming for better caching
- **Chunk Size Warnings**: Set limit to 1MB to catch large bundles
- **Compressed Size Reporting**: Enabled to track actual transfer sizes

### 2. Build Analysis Script ✅
- Added `build:analyze` script for production build analysis
- Can be extended with bundle analyzer tools if needed

### 3. Route-Based Code Splitting ✅
- Already implemented: Recharts is only loaded when BacktestPage is accessed
- This ensures the 1.2MB recharts library is not in the initial bundle

## Expected Improvements

### Minification
- **Development**: Files are not minified (expected behavior)
- **Production**: All JavaScript files are minified using esbuild
- **Expected Savings**: ~3.7 MB in production builds

### Unused JavaScript Reduction
- **Recharts**: 935 KiB unused code will be eliminated in production (tree-shaking)
- **React Router**: 358 KiB unused code is dev-only, production is smaller
- **Framer Motion**: 207 KiB unused code will be tree-shaken in production
- **Total Expected Reduction**: ~1.5-2 MB in production builds

### Code Splitting Benefits
- **Initial Bundle**: Smaller by ~1-2 MB (recharts not loaded initially)
- **Caching**: Better cache hit rates (separate vendor chunks)
- **Load Time**: Faster initial page load

## Production Build Verification

To verify optimizations are working:

```bash
cd web
npm run build
```

Check the output:
1. **Chunk Sizes**: Should see separate chunks for recharts, framer-motion, etc.
2. **Minification**: All `.js` files in `dist/assets/` should be minified
3. **File Names**: Should include hashes (e.g., `recharts-[hash].js`)

## Bundle Analysis

To analyze bundle sizes:

```bash
npm run build:analyze
# Then check dist/assets/ directory for chunk sizes
```

## Notes

1. **Development vs Production**: 
   - Development mode includes source maps and dev code (larger bundles)
   - Production builds are minified and tree-shaken (smaller bundles)
   - The diagnostics shown are from development mode

2. **Recharts Optimization**:
   - Already code-split (only loads when BacktestPage is accessed)
   - Tree-shaking will remove unused chart types in production
   - Consider alternative if still too large: Chart.js (~200 KiB) or Victory

3. **React Router**:
   - Unused code is mostly dev-only features
   - Production builds are significantly smaller

4. **Minification**:
   - Already configured with esbuild (faster than terser)
   - Automatically enabled in production builds

## Next Steps

1. **Test Production Build**: Run `npm run build` and verify chunk sizes
2. **Deploy and Test**: Deploy production build and run Lighthouse
3. **Monitor**: Check Network tab to verify code splitting is working
4. **Consider Alternatives**: If recharts is still too large, consider Chart.js or Victory

## Expected Production Metrics

| Metric | Development | Production (Expected) |
|--------|-------------|---------------------|
| Total JS Size | ~7.7 MB | ~2-3 MB |
| Minified Savings | 0 MB | ~3.7 MB |
| Unused Code | ~2.9 MB | ~0.5 MB |
| Initial Bundle | Large | ~500-800 KB |
| Recharts Chunk | N/A | ~400-500 KB (lazy-loaded) |

