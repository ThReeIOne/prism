import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { getTrace, type Trace, type Span } from '../lib/api'
import { formatDuration, formatTimestamp, kindBadgeColor, statusColor } from '../lib/format'

interface SpanNode extends Span {
  children: SpanNode[]
  depth: number
}

function buildTree(spans: Span[]): SpanNode[] {
  const map = new Map<string, SpanNode>()
  const roots: SpanNode[] = []

  for (const s of spans) {
    map.set(s.span_id, { ...s, children: [], depth: 0 })
  }

  for (const node of map.values()) {
    if (node.parent_span_id && map.has(node.parent_span_id)) {
      const parent = map.get(node.parent_span_id)!
      node.depth = parent.depth + 1
      parent.children.push(node)
    } else {
      roots.push(node)
    }
  }

  // Sort children by start time
  function sortChildren(nodes: SpanNode[]) {
    nodes.sort((a, b) => a.start_us - b.start_us)
    for (const n of nodes) sortChildren(n.children)
  }
  sortChildren(roots)
  return roots
}

function flatten(nodes: SpanNode[]): SpanNode[] {
  const result: SpanNode[] = []
  function walk(list: SpanNode[]) {
    for (const n of list) {
      result.push(n)
      walk(n.children)
    }
  }
  walk(nodes)
  return result
}

export default function TraceDetail() {
  const { traceId } = useParams<{ traceId: string }>()
  const [trace, setTrace] = useState<Trace | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [selected, setSelected] = useState<string | null>(null)

  useEffect(() => {
    if (!traceId) return
    getTrace(traceId)
      .then(setTrace)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [traceId])

  if (loading) return <div className="p-6 text-slate-500">Loading...</div>
  if (error) return <div className="p-6 text-red-400">{error}</div>
  if (!trace) return <div className="p-6 text-slate-500">Trace not found.</div>

  const tree = buildTree(trace.spans)
  const flat = flatten(tree)
  const minStart = Math.min(...trace.spans.map((s) => s.start_us))
  const maxEnd = Math.max(...trace.spans.map((s) => s.start_us + s.duration_us))
  const totalRange = maxEnd - minStart || 1
  const selectedSpan = trace.spans.find((s) => s.span_id === selected)

  return (
    <div className="p-6">
      <div className="flex items-center gap-3 mb-6">
        <Link to="/traces" className="text-slate-400 hover:text-slate-200">&larr;</Link>
        <h2 className="text-2xl font-semibold">Trace Detail</h2>
      </div>

      {/* Summary */}
      <div className="grid grid-cols-4 gap-4 mb-6">
        <InfoCard label="Trace ID" value={trace.trace_id} mono />
        <InfoCard label="Spans" value={trace.spans.length} />
        <InfoCard label="Services" value={[...new Set(trace.spans.map((s) => s.service))].length} />
        <InfoCard label="Duration" value={formatDuration(totalRange)} />
      </div>

      <div className="flex gap-4">
        {/* Waterfall */}
        <div className="flex-1 bg-slate-800 rounded-lg border border-slate-700 overflow-hidden">
          <div className="px-4 py-3 border-b border-slate-700 text-sm font-medium">Timeline</div>
          <div className="overflow-auto">
            {flat.map((span) => {
              const left = ((span.start_us - minStart) / totalRange) * 100
              const width = Math.max((span.duration_us / totalRange) * 100, 0.3)
              const isError = span.status === 'error'
              const isSelected = span.span_id === selected
              return (
                <div
                  key={span.span_id}
                  onClick={() => setSelected(span.span_id === selected ? null : span.span_id)}
                  className={`flex items-center border-b border-slate-700/30 cursor-pointer hover:bg-slate-700/30 ${
                    isSelected ? 'bg-slate-700/50' : ''
                  }`}
                  style={{ minHeight: 32 }}
                >
                  {/* Label */}
                  <div
                    className="shrink-0 px-2 py-1 text-xs truncate"
                    style={{ width: 280, paddingLeft: 12 + span.depth * 16 }}
                  >
                    <span className="text-slate-500">{span.service}</span>
                    <span className="text-slate-600 mx-1">&middot;</span>
                    <span className={isError ? 'text-red-400' : 'text-slate-300'}>{span.operation}</span>
                  </div>
                  {/* Bar */}
                  <div className="flex-1 relative h-5 mx-2">
                    <div
                      className={`absolute h-full rounded-sm ${isError ? 'bg-red-500/70' : 'bg-violet-500/70'}`}
                      style={{ left: `${left}%`, width: `${width}%`, minWidth: 2 }}
                    />
                  </div>
                  {/* Duration */}
                  <div className="shrink-0 w-20 text-right text-xs font-mono text-slate-400 pr-3">
                    {formatDuration(span.duration_us)}
                  </div>
                </div>
              )
            })}
          </div>
        </div>

        {/* Detail panel */}
        {selectedSpan && (
          <div className="w-80 shrink-0 bg-slate-800 rounded-lg border border-slate-700 overflow-auto max-h-[70vh]">
            <div className="px-4 py-3 border-b border-slate-700 text-sm font-medium flex justify-between">
              <span>Span Detail</span>
              <button onClick={() => setSelected(null)} className="text-slate-500 hover:text-slate-300">&times;</button>
            </div>
            <div className="p-4 space-y-3 text-sm">
              <Field label="Operation" value={selectedSpan.operation} />
              <Field label="Service" value={selectedSpan.service} />
              <Field label="Kind">
                <span className={`px-1.5 py-0.5 rounded text-xs ${kindBadgeColor(selectedSpan.kind)}`}>
                  {selectedSpan.kind}
                </span>
              </Field>
              <Field label="Status">
                <span className={statusColor(selectedSpan.status)}>{selectedSpan.status}</span>
              </Field>
              <Field label="Duration" value={formatDuration(selectedSpan.duration_us)} />
              <Field label="Start" value={formatTimestamp(selectedSpan.start_us)} />
              <Field label="Span ID" value={selectedSpan.span_id} mono />
              <Field label="Parent ID" value={selectedSpan.parent_span_id || '(root)'} mono />

              {selectedSpan.tags && Object.keys(selectedSpan.tags).length > 0 && (
                <div>
                  <p className="text-xs text-slate-500 mb-1">Tags</p>
                  <div className="space-y-1">
                    {Object.entries(selectedSpan.tags).map(([k, v]) => (
                      <div key={k} className="flex text-xs">
                        <span className="text-slate-400 shrink-0">{k}:</span>
                        <span className="ml-1 text-slate-300 break-all">{v}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function InfoCard({ label, value, mono }: { label: string; value: string | number; mono?: boolean }) {
  return (
    <div className="bg-slate-800 rounded-lg border border-slate-700 p-3">
      <p className="text-xs text-slate-500">{label}</p>
      <p className={`mt-0.5 text-sm truncate ${mono ? 'font-mono text-xs' : 'font-semibold'}`}>{value}</p>
    </div>
  )
}

function Field({ label, value, mono, children }: { label: string; value?: string; mono?: boolean; children?: React.ReactNode }) {
  return (
    <div>
      <p className="text-xs text-slate-500">{label}</p>
      {children || <p className={`text-slate-200 ${mono ? 'font-mono text-xs break-all' : ''}`}>{value}</p>}
    </div>
  )
}
