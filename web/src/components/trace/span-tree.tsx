import { useState } from 'react'
import { ChevronRight, ChevronDown } from 'lucide-react'
import { cn } from '@/lib/utils'
import { KindBadge } from '@/components/shared/kind-badge'
import { StatusBadge } from '@/components/shared/status-badge'
import type { Span } from '@/types'

interface SpanTreeProps {
  spans: Span[]
  selectedSpanId?: string
  onSelectSpan: (span: Span) => void
}

interface TreeNode {
  span: Span
  children: TreeNode[]
}

function buildTree(spans: Span[]): TreeNode[] {
  const map = new Map<string, TreeNode>()
  const roots: TreeNode[] = []

  spans.forEach((s) => map.set(s.span_id, { span: s, children: [] }))

  spans.forEach((s) => {
    const node = map.get(s.span_id)!
    if (s.parent_id && map.has(s.parent_id)) {
      map.get(s.parent_id)!.children.push(node)
    } else {
      roots.push(node)
    }
  })

  return roots
}

function TreeNodeRow({
  node,
  depth,
  selectedSpanId,
  onSelectSpan,
}: {
  node: TreeNode
  depth: number
  selectedSpanId?: string
  onSelectSpan: (span: Span) => void
}) {
  const [expanded, setExpanded] = useState(true)
  const hasChildren = node.children.length > 0

  return (
    <>
      <button
        className={cn(
          'flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors',
          selectedSpanId === node.span.span_id ? 'bg-primary/15 text-primary' : 'hover:bg-secondary'
        )}
        style={{ paddingLeft: `${depth * 20 + 8}px` }}
        onClick={() => onSelectSpan(node.span)}
      >
        {hasChildren ? (
          <span
            onClick={(e) => { e.stopPropagation(); setExpanded(!expanded) }}
            className="p-0.5 text-muted-foreground hover:text-foreground"
          >
            {expanded ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
          </span>
        ) : (
          <span className="w-4" />
        )}
        <span className="truncate font-medium">{node.span.name}</span>
        <KindBadge kind={node.span.kind} />
        <StatusBadge status={node.span.status} />
        {node.span.metrics?.latency_ms && (
          <span className="ml-auto font-mono text-xs text-muted-foreground">
            {node.span.metrics.latency_ms.toFixed(0)}ms
          </span>
        )}
      </button>
      {expanded &&
        node.children.map((child) => (
          <TreeNodeRow
            key={child.span.span_id}
            node={child}
            depth={depth + 1}
            selectedSpanId={selectedSpanId}
            onSelectSpan={onSelectSpan}
          />
        ))}
    </>
  )
}

export function SpanTree({ spans, selectedSpanId, onSelectSpan }: SpanTreeProps) {
  const roots = buildTree(spans)

  if (roots.length === 0) {
    return <div className="p-4 text-sm text-muted-foreground">No spans in this trace.</div>
  }

  return (
    <div className="space-y-0.5">
      {roots.map((root) => (
        <TreeNodeRow
          key={root.span.span_id}
          node={root}
          depth={0}
          selectedSpanId={selectedSpanId}
          onSelectSpan={onSelectSpan}
        />
      ))}
    </div>
  )
}
