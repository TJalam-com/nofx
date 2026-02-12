/* eslint-env node */
import sharp from 'sharp'
import { readdir, stat } from 'fs/promises'
import { join } from 'path'
import { fileURLToPath } from 'url'
import { dirname } from 'path'

const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)

const imagesDir = join(__dirname, '../public/images')

async function convertImage(inputPath, outputPath) {
  try {
    const inputStats = await stat(inputPath)
    const inputSize = inputStats.size

    await sharp(inputPath)
      .webp({ quality: 85 })
      .toFile(outputPath)

    const outputStats = await stat(outputPath)
    const outputSize = outputStats.size
    const savings = ((inputSize - outputSize) / inputSize * 100).toFixed(1)

    return {
      success: true,
      inputSize,
      outputSize,
      savings: parseFloat(savings),
    }
  } catch (error) {
    return {
      success: false,
      error: error.message,
    }
  }
}

function formatBytes(bytes) {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i]
}

async function main() {
  console.log('🖼️  Converting PNG images to WebP format...\n')

  try {
    const files = await readdir(imagesDir)
    const pngFiles = files.filter((file) => file.endsWith('.png'))

    if (pngFiles.length === 0) {
      console.log('No PNG files found in public/images/')
      return
    }

    let totalInputSize = 0
    let totalOutputSize = 0
    const results = []

    for (const pngFile of pngFiles) {
      const inputPath = join(imagesDir, pngFile)
      const webpFile = pngFile.replace('.png', '.webp')
      const outputPath = join(imagesDir, webpFile)

      console.log(`Converting ${pngFile}...`)
      const result = await convertImage(inputPath, outputPath)

      if (result.success) {
        totalInputSize += result.inputSize
        totalOutputSize += result.outputSize
        results.push({
          file: pngFile,
          ...result,
        })

        console.log(
          `  ✅ ${webpFile} created (${formatBytes(result.inputSize)} → ${formatBytes(result.outputSize)}, ${result.savings}% smaller)\n`
        )
      } else {
        console.log(`  ❌ Failed: ${result.error}\n`)
        results.push({
          file: pngFile,
          ...result,
        })
      }
    }

    // Summary
    console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━')
    console.log('📊 Conversion Summary:')
    console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n')

    results.forEach((result) => {
      if (result.success) {
        console.log(
          `${result.file.padEnd(20)} ${formatBytes(result.inputSize).padStart(10)} → ${formatBytes(result.outputSize).padStart(10)} (${result.savings.toFixed(1)}% smaller)`
        )
      } else {
        console.log(`${result.file.padEnd(20)} ❌ Failed: ${result.error}`)
      }
    })

    if (totalInputSize > 0) {
      const totalSavings = ((totalInputSize - totalOutputSize) / totalInputSize * 100).toFixed(1)
      console.log('\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━')
      console.log(`Total: ${formatBytes(totalInputSize)} → ${formatBytes(totalOutputSize)}`)
      console.log(`Total savings: ${formatBytes(totalInputSize - totalOutputSize)} (${totalSavings}%)`)
      console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n')
    }

    console.log('✅ Image conversion complete!')
    console.log('💡 Remember to update code references to use WebP with PNG fallback.')
  } catch (error) {
    console.error('❌ Error:', error.message)
    process.exit(1)
  }
}

main()
