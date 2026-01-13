import { useEditor, EditorContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Image from '@tiptap/extension-image'
import Link from '@tiptap/extension-link'
import { Bold, Italic, List, ListOrdered, Quote, Heading1, Heading2, Link as LinkIcon, Image as ImageIcon, Undo, Redo } from 'lucide-react'
import { useState } from 'react'
import { extractDirectImageUrl, convertImgBBUrl } from '../../utils/imgbb'

interface ArticleEditorProps {
  content: string
  onChange: (content: string) => void
  placeholder?: string
}

export function ArticleEditor({ content, onChange }: ArticleEditorProps) {
  const [showImageDialog, setShowImageDialog] = useState(false)
  const [imageUrl, setImageUrl] = useState('')
  const [showLinkDialog, setShowLinkDialog] = useState(false)
  const [linkUrl, setLinkUrl] = useState('')

  const editor = useEditor({
    extensions: [
      StarterKit,
      Image.configure({
        inline: true,
        allowBase64: false,
      }),
      Link.configure({
        openOnClick: false,
        HTMLAttributes: {
          class: 'text-blue-500 hover:text-blue-700 underline',
        },
      }),
    ],
    content,
    onUpdate: ({ editor }) => {
      onChange(editor.getHTML())
    },
    editorProps: {
      attributes: {
        class: 'prose prose-sm sm:prose lg:prose-lg xl:prose-xl max-w-none focus:outline-none min-h-[400px] p-4',
        style: 'color: #000000;',
      },
      handleDOMEvents: {
        // Ensure focus is maintained when clicking toolbar buttons
        mousedown: (_view, _event) => {
          // Allow the editor to maintain focus
          return false
        },
        // Handle paste events to extract ImgBB direct URLs from embed codes
        paste: (view, event) => {
          const clipboardData = event.clipboardData
          if (!clipboardData) return false

          const pastedText = clipboardData.getData('text/plain')
          if (!pastedText) return false

          // Check if pasted text contains ImgBB embed code or URL
          if (pastedText.includes('ibb.co') || pastedText.includes('<img') || pastedText.includes('[img]')) {
            const directUrl = extractDirectImageUrl(pastedText)
            if (directUrl) {
              // If we found a direct image URL, insert it as an image
              event.preventDefault()
              editor.chain().focus().setImage({ src: directUrl }).run()
              return true
            } else {
              // Try to convert if it's a page URL
              const convertedUrl = convertImgBBUrl(pastedText)
              if (convertedUrl !== pastedText && convertedUrl.includes('i.ibb.co')) {
                event.preventDefault()
                editor.chain().focus().setImage({ src: convertedUrl }).run()
                return true
              }
            }
          }
          return false
        },
      },
    },
  })

  const insertImage = () => {
    if (imageUrl && editor) {
      // Extract direct image URL from ImgBB embed codes or convert page URLs
      const directUrl = extractDirectImageUrl(imageUrl) || convertImgBBUrl(imageUrl)
      editor.chain().focus().setImage({ src: directUrl }).run()
      setImageUrl('')
      setShowImageDialog(false)
    }
  }

  const insertLink = () => {
    if (linkUrl && editor) {
      const { from, to } = editor.state.selection
      const text = editor.state.doc.textBetween(from, to)
      
      if (text) {
        editor.chain().focus().extendMarkRange('link').setLink({ href: linkUrl }).run()
      } else {
        editor.chain().focus().insertContent(`<a href="${linkUrl}">${linkUrl}</a>`).run()
      }
      setLinkUrl('')
      setShowLinkDialog(false)
    }
  }

  if (!editor) {
    return null
  }

  return (
    <div className="rounded-md" style={{ borderColor: 'var(--border-color, #d1d5db)' }}>
      {/* Toolbar */}
      <div 
        className="flex flex-wrap items-center gap-1 p-2 border-b sticky top-0 z-10 rounded-t-md" 
        style={{ 
          borderColor: 'var(--border-color, #e5e7eb)', 
          backgroundColor: 'var(--bg-secondary, #f9fafb)',
          position: 'sticky',
          top: 0,
          zIndex: 10,
        }}
      >
        <button
          type="button"
          onClick={(e) => {
            e.preventDefault()
            editor.chain().focus().toggleBold().run()
          }}
          className={`p-2 rounded-md transition-colors ${editor.isActive('bold') ? 'bg-blue-600 text-white' : 'hover:bg-gray-200'}`}
          style={{ color: editor.isActive('bold') ? 'white' : '#000000' }}
          title="Bold"
        >
          <Bold size={18} />
        </button>
        <button
          type="button"
          onClick={(e) => {
            e.preventDefault()
            editor.chain().focus().toggleItalic().run()
          }}
          className={`p-2 rounded-md transition-colors ${editor.isActive('italic') ? 'bg-blue-600 text-white' : 'hover:bg-gray-200'}`}
          style={{ color: editor.isActive('italic') ? 'white' : '#000000' }}
          title="Italic"
        >
          <Italic size={18} />
        </button>
        <div className="w-px h-6" style={{ backgroundColor: 'var(--border-color, #d1d5db)' }} />
        <button
          type="button"
          onClick={() => editor.chain().focus().toggleHeading({ level: 1 }).run()}
          className={`p-2 rounded-md transition-colors ${editor.isActive('heading', { level: 1 }) ? 'bg-blue-600 text-white' : 'hover:bg-gray-200'}`}
          style={{ color: editor.isActive('heading', { level: 1 }) ? 'white' : '#000000' }}
        >
          <Heading1 size={18} />
        </button>
        <button
          type="button"
          onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}
          className={`p-2 rounded-md transition-colors ${editor.isActive('heading', { level: 2 }) ? 'bg-blue-600 text-white' : 'hover:bg-gray-200'}`}
          style={{ color: editor.isActive('heading', { level: 2 }) ? 'white' : '#000000' }}
        >
          <Heading2 size={18} />
        </button>
        <div className="w-px h-6" style={{ backgroundColor: 'var(--border-color, #d1d5db)' }} />
        <button
          type="button"
          onClick={() => editor.chain().focus().toggleBulletList().run()}
          className={`p-2 rounded-md transition-colors ${editor.isActive('bulletList') ? 'bg-blue-600 text-white' : 'hover:bg-gray-200'}`}
          style={{ color: editor.isActive('bulletList') ? 'white' : '#000000' }}
        >
          <List size={18} />
        </button>
        <button
          type="button"
          onClick={() => editor.chain().focus().toggleOrderedList().run()}
          className={`p-2 rounded-md transition-colors ${editor.isActive('orderedList') ? 'bg-blue-600 text-white' : 'hover:bg-gray-200'}`}
          style={{ color: editor.isActive('orderedList') ? 'white' : '#000000' }}
        >
          <ListOrdered size={18} />
        </button>
        <button
          type="button"
          onClick={() => editor.chain().focus().toggleBlockquote().run()}
          className={`p-2 rounded-md transition-colors ${editor.isActive('blockquote') ? 'bg-blue-600 text-white' : 'hover:bg-gray-200'}`}
          style={{ color: editor.isActive('blockquote') ? 'white' : '#000000' }}
        >
          <Quote size={18} />
        </button>
        <div className="w-px h-6" style={{ backgroundColor: 'var(--border-color, #d1d5db)' }} />
        <button
          type="button"
          onClick={() => setShowLinkDialog(true)}
          className="p-2 rounded-md hover:bg-gray-200 transition-colors"
          style={{ color: '#000000' }}
        >
          <LinkIcon size={18} />
        </button>
        <button
          type="button"
          onClick={() => setShowImageDialog(true)}
          className="p-2 rounded-md hover:bg-gray-200 transition-colors"
          style={{ color: '#000000' }}
        >
          <ImageIcon size={18} />
        </button>
        <div className="w-px h-6" style={{ backgroundColor: 'var(--border-color, #d1d5db)' }} />
        <button
          type="button"
          onClick={() => editor.chain().focus().undo().run()}
          disabled={!editor.can().chain().focus().undo().run()}
          className="p-2 rounded-md hover:bg-gray-200 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          style={{ color: '#000000' }}
        >
          <Undo size={18} />
        </button>
        <button
          type="button"
          onClick={() => editor.chain().focus().redo().run()}
          disabled={!editor.can().chain().focus().redo().run()}
          className="p-2 rounded-md hover:bg-gray-200 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          style={{ color: '#000000' }}
        >
          <Redo size={18} />
        </button>
      </div>

      {/* Editor Content */}
      <div 
        className="min-h-[400px] p-4 rounded-b-md" 
        style={{ backgroundColor: 'var(--bg-primary, #ffffff)' }}
        onClick={() => editor.commands.focus()}
      >
        <EditorContent editor={editor} />
      </div>

      {/* Image Dialog */}
      {showImageDialog && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 max-w-md w-full mx-4 shadow-xl" style={{ backgroundColor: 'var(--bg-primary, #ffffff)' }}>
            <h3 className="text-lg font-semibold mb-2" style={{ color: 'var(--text-primary, #1f2937)' }}>Insert Image</h3>
            <p className="text-xs mb-4" style={{ color: 'var(--text-secondary, #6b7280)' }}>
              Recommended dimensions: 1200 x 900 px (preferred) or 900 x 600 px (acceptable) for optimal display quality
            </p>
            <p className="text-xs mb-4" style={{ color: 'var(--text-secondary, #6b7280)' }}>
              <strong>For ImgBB images:</strong> Paste the direct image URL from ImgBB's embed codes (HTML or BBCode) for best results. 
              You can also paste the page URL (https://ibb.co/...) and it will be converted automatically.
            </p>
            <input
              type="url"
              value={imageUrl}
              onChange={(e) => {
                setImageUrl(e.target.value)
              }}
              onPaste={(e) => {
                // Extract direct URL from pasted embed code
                const pastedText = e.clipboardData.getData('text/plain')
                if (pastedText) {
                  const directUrl = extractDirectImageUrl(pastedText) || convertImgBBUrl(pastedText)
                  if (directUrl !== pastedText || directUrl.includes('i.ibb.co')) {
                    e.preventDefault()
                    setImageUrl(directUrl)
                  }
                }
              }}
              placeholder="Enter image URL or paste ImgBB embed code"
              className="w-full px-3 py-2 border rounded-md mb-4 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors"
              style={{ 
                borderColor: 'var(--border-color, #d1d5db)', 
                color: 'var(--text-primary, #111827)', 
                backgroundColor: 'var(--bg-primary, #ffffff)' 
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  insertImage()
                }
                if (e.key === 'Escape') {
                  setShowImageDialog(false)
                }
              }}
              autoFocus
            />
            <div className="flex gap-2 justify-end">
              <button
                type="button"
                onClick={() => setShowImageDialog(false)}
                className="px-4 py-2 border rounded-md hover:bg-gray-50 transition-colors font-medium"
                style={{ 
                  borderColor: 'var(--border-color, #d1d5db)', 
                  color: 'var(--text-primary, #374151)' 
                }}
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={insertImage}
                className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors font-medium shadow-sm"
              >
                Insert
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Link Dialog */}
      {showLinkDialog && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 max-w-md w-full mx-4 shadow-xl" style={{ backgroundColor: 'var(--bg-primary, #ffffff)' }}>
            <h3 className="text-lg font-semibold mb-4" style={{ color: 'var(--text-primary, #1f2937)' }}>Insert Link</h3>
            <input
              type="url"
              value={linkUrl}
              onChange={(e) => setLinkUrl(e.target.value)}
              placeholder="Enter URL"
              className="w-full px-3 py-2 border rounded-md mb-4 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors"
              style={{ 
                borderColor: 'var(--border-color, #d1d5db)', 
                color: 'var(--text-primary, #111827)', 
                backgroundColor: 'var(--bg-primary, #ffffff)' 
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  insertLink()
                }
                if (e.key === 'Escape') {
                  setShowLinkDialog(false)
                }
              }}
              autoFocus
            />
            <div className="flex gap-2 justify-end">
              <button
                type="button"
                onClick={() => setShowLinkDialog(false)}
                className="px-4 py-2 border rounded-md hover:bg-gray-50 transition-colors font-medium"
                style={{ 
                  borderColor: 'var(--border-color, #d1d5db)', 
                  color: 'var(--text-primary, #374151)' 
                }}
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={insertLink}
                className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors font-medium shadow-sm"
              >
                Insert
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
