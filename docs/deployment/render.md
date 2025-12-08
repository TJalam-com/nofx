# Render Deployment Guide

## Prerequisites

1. Render account (sign up at https://render.com)
2. GitHub/GitLab repository connected to Render
3. Production branch ready

## Deployment Steps

### 1. Create Web Service

1. Go to Render Dashboard
2. Click "New +" → "Web Service"
3. Connect your repository
4. Select branch: `production`

### 2. Configure Service

**Root Directory:** (leave empty)

**Build Command:**
```bash
cd web && npm ci && npm run build && cd .. && go mod download && go build -trimpath -ldflags="-s -w" -o nofx .
```

**Start Command:**
```bash
./nofx
```

**Alternative:** You can use the `render.yaml` file for automatic configuration. Render will detect it and use the settings defined there.

### 3. Environment Variables

Set these in Render Dashboard → Environment:

- `JWT_SECRET` - Your JWT secret (generate a strong random key, minimum 32 characters)
- `DATA_ENCRYPTION_KEY` - Your encryption key (generate a strong random key, minimum 32 characters)
- `GIN_MODE=release`
- `TZ=UTC` (or your preferred timezone)
- `AI_MAX_TOKENS=4000` (optional, defaults to 4000)

**Note:** Render automatically provides `$PORT` - your app will use it automatically. The application checks for `$PORT` first, then falls back to `NOFX_BACKEND_PORT` if needed.

### 4. Persistent Disk (Optional but Recommended)

For data persistence (config.db, decision_logs, etc.):

1. Go to your service → "Disks"
2. Add a new disk (1GB minimum)
3. Mount path: `/opt/render/project/src`

This ensures your database and logs persist across deployments.

### 5. Health Check

Render will automatically check: `/api/health`

The health check endpoint returns:
```json
{
  "status": "ok",
  "time": "..."
}
```

## Production Checklist

- [ ] All environment variables set
- [ ] JWT_SECRET is strong and random (min 32 chars)
- [ ] DATA_ENCRYPTION_KEY is strong and random (min 32 chars)
- [ ] Persistent disk configured (if needed)
- [ ] Health check endpoint working
- [ ] Frontend builds successfully
- [ ] Backend serves static files correctly
- [ ] Test the application after deployment

## Environment Variables Reference

### Required

- `JWT_SECRET` - JWT authentication secret key
- `DATA_ENCRYPTION_KEY` - Database encryption key

### Optional

- `NOFX_BACKEND_PORT` - Backend port (defaults to Render's $PORT)
- `GIN_MODE` - Gin framework mode (set to `release` for production)
- `TZ` - Timezone (default: UTC)
- `AI_MAX_TOKENS` - Maximum AI response tokens (default: 4000)

## Build Process

The build command performs the following steps:

1. **Frontend Build:**
   - Installs npm dependencies (`npm ci`)
   - Builds the React frontend (`npm run build`)
   - Output: `web/dist/`

2. **Backend Build:**
   - Downloads Go dependencies (`go mod download`)
   - Builds optimized Go binary (`go build -trimpath -ldflags="-s -w"`)
   - Output: `nofx` executable

3. **Static File Serving:**
   - The backend automatically serves static files from `web/dist/` if it exists
   - Frontend routes are handled by serving `index.html` for SPA routing
   - API routes (`/api/*`) are handled by the backend API

## Troubleshooting

### Build Fails

**Issue:** Build command fails with errors

**Solutions:**
- Check build logs in Render dashboard
- Ensure Node.js and Go are available in the build environment
- Verify all dependencies are in `go.mod` and `package.json`
- Check that the `web/` directory exists and contains `package.json`

### App Won't Start

**Issue:** Application fails to start after successful build

**Solutions:**
- Check start command is correct: `./nofx`
- Verify PORT environment variable is being used (check logs)
- Ensure all required environment variables are set
- Check application logs for specific error messages

### Static Files Not Serving

**Issue:** Frontend not loading, 404 errors for static assets

**Solutions:**
- Verify `web/dist` exists after build (check build logs)
- Ensure static file serving code is in `api/server.go`
- Check that the build command successfully creates `web/dist/`
- Verify file permissions

### Database/Data Not Persisting

**Issue:** Data is lost after redeployment

**Solutions:**
- Configure persistent disk in Render dashboard
- Set mount path to `/opt/render/project/src`
- Ensure database files are written to the mounted disk location
- Check disk size is sufficient (minimum 1GB recommended)

### Port Issues

**Issue:** Application can't bind to port

**Solutions:**
- Render automatically provides `$PORT` - don't hardcode port numbers
- The app checks `$PORT` first, then `NOFX_BACKEND_PORT`, then database config
- Check logs to see which port is being used

## Manual Deployment Commands

If you need to test the build locally before deploying:

```bash
# Build frontend
cd web && npm ci && npm run build && cd ..

# Build backend
go mod download
go build -trimpath -ldflags="-s -w" -o nofx .

# Run
./nofx
```

## Updating Deployment

1. Push changes to `production` branch
2. Render automatically detects changes and redeploys
3. Monitor deployment logs in Render dashboard
4. Verify health check passes after deployment

## Security Notes

- **Never commit** `.env` files or sensitive keys
- Use Render's environment variable management for secrets
- Generate strong random keys for `JWT_SECRET` and `DATA_ENCRYPTION_KEY`
- Use at least 32 characters for secret keys
- Enable HTTPS in Render (automatic with custom domains)

## Support

For issues specific to Render platform, consult:
- [Render Documentation](https://render.com/docs)
- [Render Community](https://community.render.com)

For application-specific issues, check:
- Application logs in Render dashboard
- Health check endpoint: `https://your-app.onrender.com/api/health`

