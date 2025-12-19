import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Generate a URL-friendly slug from trader name and ID
 * Format: "name-id4" where id4 is the last 4 characters of the trader ID
 * 
 * @param name Trader name
 * @param id Trader ID
 * @returns Slug string in format "name-id4"
 */
export function generateTraderSlug(name: string, id: string): string {
  // Sanitize name: convert to lowercase, replace spaces/special chars with hyphens
  const sanitizedName = name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
  
  // Get last 4 characters of ID
  const id4 = id.length >= 4 ? id.slice(-4) : id
  
  return `${sanitizedName}-${id4}`
}

/**
 * Parse trader slug back to trader ID
 * Note: This extracts the ID4 part, but full ID lookup should be done via API
 * 
 * @param slug Slug string in format "name-id4"
 * @returns The id4 portion (last 4 chars of trader ID)
 */
export function parseTraderSlug(slug: string): string | null {
  const parts = slug.split('-')
  if (parts.length < 2) {
    return null
  }
  // Return the last part which should be the id4
  return parts[parts.length - 1] || null
}

/**
 * Get the last 4 characters of an ID
 * 
 * @param id Trader ID
 * @returns Last 4 characters
 */
export function getLast4Chars(id: string): string {
  return id.length >= 4 ? id.slice(-4) : id
}
