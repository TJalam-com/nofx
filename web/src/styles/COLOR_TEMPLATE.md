# AI Trading 24x7 Color Template

## Color System Overview

This document provides a comprehensive guide to the AI Trading 24x7 color system. All colors are defined as CSS custom properties (variables) in `index.css` and should be used via `var(--variable-name)`.

## Primary Colors

### Navy Blue (Professional Base)
- `--navy-primary: #001F3F` - Main background, headers, primary containers
- `--navy-dark: #00152E` - Elevated backgrounds, panels, cards
- `--navy-light: #003366` - Hover states, active elements

### Bright Green (CTAs & Growth)
- `--green-primary: #00FF7F` - Primary CTAs, success states, profit indicators
- `--green-dark: #00CC66` - Hover states, pressed buttons
- `--green-light: #33FF99` - Light accents, highlights
- `--green-glow: rgba(0, 255, 127, 0.2)` - Glow effects, shadows

### White (Text & Contrast)
- `--white: #FFFFFF` - Primary text, high contrast elements

## Secondary Colors

### Grays (Subtle Elements)
- `--gray-medium: #808080` - Secondary text, dividers
- `--gray-light: #A9A9A9` - Tertiary text, placeholders
- `--gray-dark: #404040` - Borders, panel borders

### Gold (Premium Accents)
- `--gold-primary: #FFD700` - Premium highlights, warnings, testimonials
- `--gold-dark: #CCAA00` - Darker gold variant
- `--gold-light: #FFE44D` - Lighter gold variant
- `--gold-glow: rgba(255, 215, 0, 0.2)` - Gold glow effects

## Background System

### Main Backgrounds
- `--background: var(--navy-primary)` - Page background
- `--header-bg: var(--navy-primary)` - Header background
- `--background-elevated: var(--navy-dark)` - Elevated sections

### Panel Backgrounds
- `--panel-bg: var(--navy-dark)` - Card/panel background
- `--panel-bg-hover: var(--navy-light)` - Panel hover state
- `--panel-border: var(--gray-dark)` - Panel borders
- `--panel-border-hover: var(--gray-medium)` - Panel border hover

## Text Colors

- `--text-primary: var(--white)` - Main text, headings
- `--text-secondary: var(--gray-light)` - Secondary text, descriptions
- `--text-tertiary: var(--gray-medium)` - Tertiary text, hints
- `--text-disabled: var(--gray-medium)` - Disabled text

### Extended Text Palette
- `--text-gray-light: #848E9C` - Light gray text (legacy support)
- `--text-gray-medium: #A9A9A9` - Medium gray text
- `--text-gray-dark: #404040` - Dark gray text
- `--text-white: #EAECEF` - Off-white text

## Functional Colors

### Success (Green)
- `--success: var(--green-primary)` - Success messages, profit
- `--success-bg: rgba(0, 255, 127, 0.1)` - Success backgrounds
- `--success-border: rgba(0, 255, 127, 0.2)` - Success borders

### Error (Red)
- `--error: #FF4444` - Error messages, losses
- `--error-bg: rgba(255, 68, 68, 0.1)` - Error backgrounds
- `--error-border: rgba(255, 68, 68, 0.2)` - Error borders

### Warning (Gold)
- `--warning: var(--gold-primary)` - Warning messages
- `--info: var(--green-primary)` - Info messages

## Accent Colors

### Yellow Accent (Legacy/Compatibility)
- `--accent-yellow: #F0B90B` - Yellow accent (for compatibility)
- `--accent-yellow-dark: #CCAA00` - Dark yellow
- `--accent-yellow-light: #FFE44D` - Light yellow
- `--accent-yellow-bg: rgba(240, 185, 11, 0.15)` - Yellow background
- `--accent-yellow-border: rgba(240, 185, 11, 0.3)` - Yellow border
- `--accent-yellow-glow: rgba(240, 185, 11, 0.2)` - Yellow glow

### Extended Backgrounds
- `--bg-dark: #0B0E11` - Dark background
- `--bg-darker: #1E2329` - Darker background
- `--bg-darkest: #252B35` - Darkest background
- `--bg-panel: #2B3139` - Panel background

## Chart Colors

- `--grid-stroke: var(--gray-dark)` - Chart grid lines
- `--axis-tick: var(--gray-medium)` - Axis tick marks
- `--ref-line: var(--gray-medium)` - Reference lines

## Shadows

All shadows use navy-based colors for consistency:

- `--shadow-sm` - Small shadow (cards, buttons)
- `--shadow-md` - Medium shadow (hover states)
- `--shadow-lg` - Large shadow (modals, dropdowns)
- `--shadow-xl` - Extra large shadow (popovers)

## Legacy/Binance Variables (Backward Compatibility)

These variables map to the new color system for backward compatibility:

- `--brand-yellow` → `--green-primary`
- `--brand-black` → `--navy-primary`
- `--brand-dark-gray` → `--navy-dark`
- `--brand-light-gray` → `--white`
- `--binance-yellow` → `--green-primary`
- `--binance-green` → `--green-primary`
- `--binance-red` → `--error`

## Usage Guidelines

### Primary Actions
Use `--green-primary` for primary CTAs, buttons, and important actions.

```css
.primary-button {
  background: var(--green-primary);
  color: var(--navy-primary);
}
```

### Secondary Actions
Use `--gold-primary` or `--navy-dark` with borders for secondary actions.

```css
.secondary-button {
  background: var(--navy-dark);
  border: 1.5px solid var(--gold-primary);
  color: var(--white);
}
```

### Text Hierarchy
- Headings: `var(--text-primary)` (white)
- Body text: `var(--text-secondary)` (light gray)
- Hints/placeholders: `var(--text-tertiary)` (medium gray)

### Status Indicators
- Profit/Success: `var(--green-primary)`
- Loss/Error: `var(--error)`
- Warning: `var(--gold-primary)`
- Info: `var(--green-primary)`

### Cards & Panels
```css
.card {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
}

.card:hover {
  background: var(--panel-bg-hover);
  border-color: var(--panel-border-hover);
}
```

## Color Combinations

### Professional (Navy + Green)
- Background: `var(--navy-primary)`
- Text: `var(--white)`
- Accent: `var(--green-primary)`

### Premium (Navy + Gold)
- Background: `var(--navy-dark)`
- Text: `var(--white)`
- Accent: `var(--gold-primary)`

### Success State
- Background: `var(--success-bg)`
- Text: `var(--green-primary)`
- Border: `var(--success-border)`

### Error State
- Background: `var(--error-bg)`
- Text: `var(--error)`
- Border: `var(--error-border)`

## Best Practices

1. **Always use CSS variables** - Never hardcode hex colors
2. **Use semantic names** - Prefer `--success` over `--green-primary` for functional colors
3. **Maintain contrast** - Ensure WCAG AA compliance (4.5:1 for text)
4. **Consistent hover states** - Use `-hover` variants for interactive elements
5. **Test in dark mode** - All colors are optimized for dark theme

## Migration Guide

If you find hardcoded colors in components, replace them:

- `#00FF7F` → `var(--green-primary)`
- `#001F3F` → `var(--navy-primary)`
- `#00152E` → `var(--navy-dark)`
- `#FFD700` → `var(--gold-primary)`
- `#FF4444` → `var(--error)`
- `#848E9C` → `var(--text-gray-light)` or `var(--text-secondary)`
- `#EAECEF` → `var(--text-white)` or `var(--text-primary)`
- `rgba(240, 185, 11, 0.15)` → `var(--accent-yellow-bg)`

