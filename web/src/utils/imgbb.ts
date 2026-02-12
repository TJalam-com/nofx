/**
 * ImgBB URL utility functions
 * Handles conversion of ImgBB page URLs to direct image URLs
 * and extraction of direct URLs from embed codes
 */

/**
 * Extracts direct image URL from ImgBB HTML embed code
 * Example: <a href="https://ibb.co/3YccGV0P"><img src="https://i.ibb.co/3YccGV0P/image.jpg"></a>
 */
export function extractDirectUrlFromHtmlEmbed(html: string): string | null {
  if (!html) return null

  // Try to match img src in HTML embed code
  const imgSrcMatch = html.match(/<img[^>]+src=["']([^"']+)["']/i)
  if (imgSrcMatch && imgSrcMatch[1]) {
    const url = imgSrcMatch[1].trim()
    // Verify it's an ImgBB direct URL
    if (url.includes('i.ibb.co')) {
      return url
    }
  }

  return null
}

/**
 * Extracts direct image URL from ImgBB BBCode embed
 * Example: [url=https://ibb.co/3YccGV0P][img]https://i.ibb.co/3YccGV0P/image.jpg[/img][/url]
 */
export function extractDirectUrlFromBBCode(bbcode: string): string | null {
  if (!bbcode) return null

  // Try to match img tag in BBCode
  const imgMatch = bbcode.match(/\[img\]([^[]+)\[\/img\]/i)
  if (imgMatch && imgMatch[1]) {
    const url = imgMatch[1].trim()
    // Verify it's an ImgBB direct URL
    if (url.includes('i.ibb.co')) {
      return url
    }
  }

  return null
}

/**
 * Converts ImgBB page URL to direct image URL using multiple pattern attempts
 * @param pageUrl - ImgBB page URL (e.g., https://ibb.co/3YccGV0P)
 * @returns Direct image URL or original URL if conversion fails
 */
export function convertPageUrlToDirectUrl(pageUrl: string): string {
  if (!pageUrl) return pageUrl

  // If already a direct URL, return as-is
  if (pageUrl.includes('i.ibb.co')) {
    return pageUrl
  }

  // Extract image ID from page URL
  const match = pageUrl.match(/ibb\.co\/([a-zA-Z0-9]+)/)
  if (!match || !match[1]) {
    return pageUrl
  }

  const imageId = match[1]

  // Try multiple URL patterns (ImgBB uses various formats)
  // Pattern 1: https://i.ibb.co/{id}/{id}.jpg
  // Pattern 2: https://i.ibb.co/{id}/{id}.png
  // Pattern 3: https://i.ibb.co/{id}/{id}.jpeg
  // Pattern 4: https://i.ibb.co/{id}/{id} (no extension, less common)

  // Note: We can't determine the actual format without fetching the page,
  // so we'll try the most common pattern (jpg) first
  // The actual implementation should ideally fetch the page or use ImgBB API
  return `https://i.ibb.co/${imageId}/${imageId}.jpg`
}

/**
 * Validates if a URL is an ImgBB direct image URL
 */
export function isDirectImageUrl(url: string): boolean {
  if (!url) return false
  return url.includes('i.ibb.co') && /\.(jpg|jpeg|png|gif|webp|bmp)$/i.test(url)
}

/**
 * Main function to convert any ImgBB URL format to direct image URL
 * Handles:
 * - Page URLs (https://ibb.co/...)
 * - Direct URLs (already correct)
 * - HTML embed codes
 * - BBCode embed codes
 */
export function convertImgBBUrl(input: string): string {
  if (!input) return input

  const trimmed = input.trim()

  // If already a direct image URL, return as-is
  if (isDirectImageUrl(trimmed)) {
    return trimmed
  }

  // Try extracting from HTML embed code
  const htmlUrl = extractDirectUrlFromHtmlEmbed(trimmed)
  if (htmlUrl) {
    return htmlUrl
  }

  // Try extracting from BBCode
  const bbcodeUrl = extractDirectUrlFromBBCode(trimmed)
  if (bbcodeUrl) {
    return bbcodeUrl
  }

  // Try converting page URL to direct URL
  if (trimmed.includes('ibb.co/') && !trimmed.includes('i.ibb.co')) {
    return convertPageUrlToDirectUrl(trimmed)
  }

  // Return original if no conversion possible
  return trimmed
}

/**
 * Extracts direct image URL from any ImgBB format (embed codes or URLs)
 * Returns null if no valid direct URL can be extracted
 */
export function extractDirectImageUrl(input: string): string | null {
  if (!input) return null

  const trimmed = input.trim()

  // If already a direct image URL, return it
  if (isDirectImageUrl(trimmed)) {
    return trimmed
  }

  // Try extracting from HTML embed code
  const htmlUrl = extractDirectUrlFromHtmlEmbed(trimmed)
  if (htmlUrl) {
    return htmlUrl
  }

  // Try extracting from BBCode
  const bbcodeUrl = extractDirectUrlFromBBCode(trimmed)
  if (bbcodeUrl) {
    return bbcodeUrl
  }

  // For page URLs, we can't reliably get the direct URL without fetching
  // Return null to indicate we couldn't extract a direct URL
  return null
}
