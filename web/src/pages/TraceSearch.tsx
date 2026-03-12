import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { searchTraces, getServices, type Trace, type ServiceInfo } from '../lib/api'
import { formatDuration, formatTimestamp, statusColor } from '../lib/format'

export default function TraceSearch() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [services, setServices] = useState<ServiceInfo[]>([])
  const [traces, setTraces] = useState<Trace[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  // Form state from URL params
  const [service, setService] = useState(searchParams.get('service') || '')
  const [operation, setOperation] = useState(searchParams.get('operation') || '')
  const [status, setStatus] = useState(searchParams.get('status') || '')
  const [minDuration, setMinDuration] = useState(searchParams.get('min_duration') || '')
  const [limit, setLimit] = useState(searchParams.get('limit') || '20')

  useEffect(() => {
    getServices().then((r) => setServices(r.services || [])).catch((e) => setError(e.message))
  }, [])

  useEffect(() => {
    doSearch()
  }, [searchParams])

  function doSearch() {
    const params: Record<string, string> = {}
    const s = searchParams.get('service')
    const o = searchParams.get('operation')
    const st = searchParams.get('status')
    const md = searchParams.get('min_duration')
    const l = searchParams.get('limit')
    if (s) params.service = s
    if (o) params.operation = o
    if (st) params.status = st
    if (md) params.min_duration = md
    params.limit = l || '20'

    setLoading(true)
    setError('')
    searchTraces(params)
      .then((r) => setTraces(r.traces || []))
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const params: Record<string, string> = {}
    if (service) params.service = service
    if (operation) params.operation = operation
    if (status) params.status = status
    if (minDuration) params.min_duration = minDuration
    if (limit) params.limit = limit
    setSearchParams(params)
  }

  return (
    <div className="p-6">
      <h2 className="text-2xl font-semibold mb-6">Trace Search</h2>

      {/* Search form */}
      <form onSubmit={handleSubmit} className="bg-slate-800 rounded-lg border border-slate-700 p-4 mb-6">
        <div className="grid grid-cols-6 gap-3">
          <div>
            <label className="block text-xs text-slate-400 mb-1">Service</label>
            <select
              value={service}
              onChange={(e) => setService(e.target.value)}
              className="w-full bg-slate-700 border border-slate-600 rounded px-2 py-1.5 text-sm"
            >
              <option value="">All</option>
              {services.map((s) => (
                <option key={s.name} value={s.name}>{s.name}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-xs text-slate-400 mb-1">Operation</label>
            <input
              value={operation}
              onChange={(e) => setOperation(e.target.value)}
              placeholder="e.g. POST /api/orders"
              className="w-full bg-slate-700 border border-slate-600 rounded px-2 py-1.5 text-sm"
            />
          </div>
          <div>
            <label className="block text-xs text-slate-400 mb-1">Status</label>
            <select
              value={status}
              onChange={(e) => setStatus(e.target.value)}
              className="w-full bg-slate-700 border border-slate-600 rounded px-2 py-1.5 text-sm"
            >
              <option value="">All</option>
              <option value="ok">OK</option>
              <option value="error">Error</option>
            </select>
          </div>
          <div>
            <label className="block text-xs text-slate-400 mb-1">Min Duration</label>
            <input
              value={minDuration}
              onChange={(e) => setMinDuration(e.target.value)}
              placeholder="e.g. 100ms, 1s"
              className="w-full bg-slate-700 border border-slate-600 rounded px-2 py-1.5 text-sm"
            />
          </div>
          <div>
            <label className="block text-xs text-slate-400 mb-1">Limit</label>
            <select
              value={limit}
              onChange={(e) => setLimit(e.target.value)}
              className="w-full bg-slate-700 border border-slate-600 rounded px-2 py-1.5 text-sm"
            >
              <option value="20">20</option>
              <option value="50">50</option>
              <option value="100">100</option>
            </select>
          </div>
          <div className="flex items-end">
            <button
              type="submit"
              className="w-full bg-violet-600 hover:bg-violet-500 text-white rounded px-4 py-1.5 text-sm font-medium transition-colors"
            >
              Search
            </button>
          </div>
        </div>
      </form>

      {/* Results */}
      <div className="bg-slate-800 rounded-lg border border-slate-700 overflow-hidden">
        <div className="px-4 py-3 border-b border-slate-700 text-sm text-slate-400">
          {loading ? 'Searching...' : `${traces.length} trace(s) found`}
        </div>
        {error ? (
          <div className="p-8 text-center text-red-400">{error}</div>
        ) : traces.length === 0 && !loading ? (
          <div className="p-8 text-center text-slate-500">No traces found.</div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-slate-400 text-left">
                <th className="px-4 py-2 font-medium">Trace ID</th>
                <th className="px-4 py-2 font-medium">Root Operation</th>
                <th className="px-4 py-2 font-medium">Services</th>
                <th className="px-4 py-2 font-medium text-right">Duration</th>
                <th className="px-4 py-2 font-medium text-right">Spans</th>
                <th className="px-4 py-2 font-medium text-right">Time</th>
              </tr>
            </thead>
            <tbody>
              {traces.map((trace) => {
                const root = trace.spans.find((s) => !s.parent_span_id) || trace.spans[0]
                const svcs = [...new Set(trace.spans.map((s) => s.service))]
                const hasError = trace.spans.some((s) => s.status === 'error')
                const totalDuration = root ? root.duration_us : Math.max(...trace.spans.map((s) => s.duration_us))
                return (
                  <tr key={trace.trace_id} className="border-t border-slate-700/50 hover:bg-slate-700/30">
                    <td className="px-4 py-2.5">
                      <Link to={`/traces/${trace.trace_id}`} className="text-violet-400 hover:underline font-mono text-xs">
                        {trace.trace_id.slice(0, 16)}...
                      </Link>
                    </td>
                    <td className="px-4 py-2.5">
                      <span className={statusColor(hasError ? 'error' : 'ok')}>{root?.operation || '-'}</span>
                    </td>
                    <td className="px-4 py-2.5">
                      <div className="flex gap-1 flex-wrap">
                        {svcs.map((s) => (
                          <span key={s} className="bg-slate-700 px-1.5 py-0.5 rounded text-xs">{s}</span>
                        ))}
                      </div>
                    </td>
                    <td className="px-4 py-2.5 text-right font-mono">{formatDuration(totalDuration)}</td>
                    <td className="px-4 py-2.5 text-right">{trace.spans.length}</td>
                    <td className="px-4 py-2.5 text-right text-xs text-slate-400">
                      {root ? formatTimestamp(root.start_us) : '-'}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
