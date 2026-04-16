import { useState } from 'react'
import { ChevronRight, ChevronDown, Copy, Check } from 'lucide-react'
import { cn } from '@/lib/utils'

interface JsonViewerProps {
  data: unknown
  collapsed?: boolean
  label?: string
}

export function JsonViewer({ data, collapsed = true, label }: JsonViewerProps) {
  const [isCollapsed, setIsCollapsed] = useState(collapsed)
  const [copied, setCopied] = useState(false)

  const text = typeof data === 'string' ? data : JSON.stringify(data, null, 2)
  const isExpandable = typeof data === 'object' && data !== null

  const handleCopy = () => {
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  return (
    <div className="group relative">
      <div className="flex items-center gap-1">
        {isExpandable && (
          <button
            onClick={() => setIsCollapsed(!isCollapsed)}
            className="p-0.5 text-muted-foreground hover:text-foreground"
          >
            {isCollapsed ? <ChevronRight className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
          </button>
        )}
        {label && <span className="text-xs font-medium text-muted-foreground">{label}</span>}
        <button
          onClick={handleCopy}
          className="ml-auto p-1 text-muted-foreground opacity-0 transition-opacity hover:text-foreground group-hover:opacity-100"
        >
          {copied ? <Check className="h-3 w-3 text-emerald-400" /> : <Copy className="h-3 w-3" />}
        </button>
      </div>
      <pre
        className={cn(
          'mt-1 overflow-x-auto rounded-md bg-zinc-900 p-3 text-xs text-zinc-300 font-mono',
          isCollapsed && isExpandable && 'max-h-10 overflow-hidden'
        )}
      >
        {text}
      </pre>
    </div>
  )
}
