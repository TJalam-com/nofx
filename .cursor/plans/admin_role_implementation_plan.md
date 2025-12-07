# Admin Role Implementation Plan

## Overview
Create an "Admin" role that can:
- See all traders across all users
- Start/stop any trader
- Assign roles ("user" or "follower") to any user
- Access a new `/stats` page (admin-only)

## Role Access Matrix
- **Admin**: Access to all pages including `/stats`
- **User**: Access to all pages except `/stats`
- **Follower**: Same as current (no changes)

## Implementation Steps

### Backend Changes (Go)

#### 1. Create Admin Middleware (`api/server.go`)
- Add `adminOnlyMiddleware()` function similar to `nonFollowerMiddleware()`
- Check if user role is "admin"
- Return 403 if not admin

#### 2. Create Admin API Endpoints (`api/server.go`)

**Get All Traders (Admin Only)**
- Endpoint: `GET /api/admin/traders`
- Middleware: `authMiddleware()` + `adminOnlyMiddleware()`
- Returns: All traders from all users with user_id information
- Handler: `handleGetAllTraders()`

**Get All Users (Admin Only)**
- Endpoint: `GET /api/admin/users`
- Middleware: `authMiddleware()` + `adminOnlyMiddleware()`
- Returns: All users with their roles
- Handler: `handleGetAllUsers()`

**Update User Role (Admin Only)**
- Endpoint: `PUT /api/admin/users/:id/role`
- Middleware: `authMiddleware()` + `adminOnlyMiddleware()`
- Body: `{ "role": "user" | "follower" }`
- Handler: `handleUpdateUserRole()`

**Start/Stop Any Trader (Admin)**
- Modify existing endpoints: `POST /api/traders/:id/start` and `POST /api/traders/:id/stop`
- Check if user is admin OR trader belongs to user
- If admin, allow starting/stopping any trader

#### 3. Database Methods (`config/database.go`)

**Get All Traders**
- Add `GetAllTraders()` method that returns traders from all users
- Include user_id in the result

**Get All Users**
- Add `GetAllUsers()` method that returns all users with roles
- Already exists as `GetAllUsers() []string` but needs to return full user objects

**Update User Role**
- Add `UpdateUserRole(userID string, role string) error` method
- Update the role field in users table

### Frontend Changes (React/TypeScript)

#### 1. Auth Context Updates (`web/src/contexts/AuthContext.tsx`)
- Add `isAdmin(user: User | null): boolean` helper function
- Export it similar to `isFollower`

#### 2. API Client Updates (`web/src/api/` or wherever API calls are made)
- Add `getAllTraders()` function
- Add `getAllUsers()` function
- Add `updateUserRole(userId: string, role: string)` function
- Modify `startTrader()` and `stopTrader()` to work with admin permissions

#### 3. Create Stats Page (`web/src/pages/StatsPage.tsx`)
- Component showing:
  - Table of all traders with:
    - Trader ID, Name, Owner (user email), AI Model, Exchange, Status (running/stopped)
    - Start/Stop buttons for each trader
  - Table of all users with:
    - User ID, Email, Current Role
    - Role dropdown to change role (user/follower)
    - Save button to update role
- Use existing styling patterns from AITradersPage
- Responsive design with mobile support
- Dark/light mode compatible

#### 4. Routing Updates (`web/src/routes/index.tsx`)
- Add `/stats` route
- Protect with admin check (redirect if not admin)
- Use MainLayout

#### 5. HeaderBar Updates (`web/src/components/HeaderBar.tsx`)
- Add "Stats" navigation tab (admin only)
- Show between "Dashboard" and "Followers" tabs
- Hide for non-admin users
- Update mobile menu to include Stats tab (admin only)

#### 6. MainLayout Updates (`web/src/layouts/MainLayout.tsx`)
- Update `getCurrentPage()` to handle 'stats' page
- Add 'stats' to Page type

## File Changes Summary

### Backend Files
1. `api/server.go`
   - Add `adminOnlyMiddleware()`
   - Add `handleGetAllTraders()`
   - Add `handleGetAllUsers()`
   - Add `handleUpdateUserRole()`
   - Modify `handleStartTrader()` and `handleStopTrader()` for admin access
   - Add admin routes group

2. `config/database.go`
   - Add `GetAllTraders() ([]*TraderRecord, error)`
   - Modify `GetAllUsers()` to return full user objects
   - Add `UpdateUserRole(userID string, role string) error`

### Frontend Files
1. `web/src/contexts/AuthContext.tsx`
   - Add `isAdmin()` helper function

2. `web/src/pages/StatsPage.tsx` (NEW)
   - Create new Stats page component

3. `web/src/routes/index.tsx`
   - Add `/stats` route with admin protection

4. `web/src/components/HeaderBar.tsx`
   - Add Stats navigation tab (admin only)
   - Update mobile menu

5. `web/src/layouts/MainLayout.tsx`
   - Update Page type and getCurrentPage function

6. API client files (need to locate)
   - Add admin API functions

## Security Considerations
1. All admin endpoints must check role in middleware
2. Frontend route protection should redirect non-admins
3. Admin actions should be logged for audit purposes
4. Validate role values ("user", "follower", "admin") on backend

## Testing Checklist
- [ ] Admin can access `/stats` page
- [ ] Non-admin users cannot access `/stats` (redirected)
- [ ] Admin can see all traders
- [ ] Admin can start/stop any trader
- [ ] Admin can view all users
- [ ] Admin can change user roles
- [ ] Regular users cannot access admin endpoints (403)
- [ ] Follower role behavior unchanged
- [ ] Mobile navigation shows Stats for admin only




