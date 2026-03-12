import { useEffect, useState } from 'react'
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip,
  ResponsiveContainer, AreaChart, Area, Legend,
} from 'recharts'
import {
  getServices, getLatencyStats, getThroughputStats,
  type ServiceInfo, type LatencyPoint, type ThroughputPoint,
} from '../lib/api'

interface LatencyChartPoint {
  time: string
  p50: number
  p90: number
  p99: number
  max: number
}

interface ThroughputChartPoint {
  time: string
  total: number
  errors: number
  errorRate: number
}

const RANGES = [
  { label: '1h', value: '1h' },
  { label: '6h', value: '6h' },
  { label: '24h', value: '24h' },
]

export default function Stats() {
  const [services, setServices] = useState<ServiceInfo[]>([])
  const [service, setService] = useState('')
  const [range, setRange] = useState('1h')
  const [latency, setLatency] = useState<LatencyChartPoint[]>([])
  const [throughput, setThroughput] = useState<ThroughputChartPoint[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    getServices()
      .then((r) => {
        const svcs = r.services || []
        setServices(svcs)
        if (svcs.length > 0 && !service) setService(svcs[0].name)
      })
      .catch(() => {})
  }, [])

  useEffect(() => {
    if (!service) return
    fetchStats()
  }, [service, range])

  function fetchStats() {
    setLoading(true)
    setError('')
    const now = new Date()
    const hours = range === '1h' ? 1 : range === '6h' ? 6 : 24
    const start = new Date(now.getTime() - hours * 3600_000).toISOString()
    const end = now.toISOString()
    const granularity = hours <= 1 ? '1m' : hours <= 6 ? '5m' : '15m'
    const params = { service, start, end, granularity }

    Promise.all([
      getLatencyStats(params).catch(() => ({ data: [] })),
      getThroughputStats(params).catch(() => ({ data: [] })),
    ])
      .then(([lat, thr]) => {
        setLatency((lat.data || []).map(formatLatencyPoint))
        setThroughput((thr.data || []).map(formatThroughputPoint))
      })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }

  return (
    <div className="p-6">
      <h2 className="text-2xl font-semibold mb-6">Statistics</h2>

      {/* Controls */}
      <div className="flex items-center gap-4 mb-6">
        <div>
          <label className="block text-xs text-slate-400 mb-1">Service</label>
          <select
            value={service}
            onChange={(e) => setService(e.target.value)}
            className="bg-slate-700 border border-slate-600 rounded px-3 py-1.5 text-sm min-w-[180px]"
          >
            {services.map((s) => (
              <option key={s.name} value={s.name}>{s.name}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-xs text-slate-400 mb-1">Time Range</label>
          <div className="flex gap-1">
            {RANGES.map((r) => (
              <button
                key={r.value}
                onClick={() => setRange(r.value)}
                className={`px-3 py-1.5 text-sm rounded transition-colors ${
                  range === r.value
                    ? 'bg-violet-600 text-white'
                    : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
                }`}
              >
                {r.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {error && <div className="mb-4 text-red-400 text-sm">{error}</div>}

      {loading ? (
        <div className="text-slate-500">Loading...</div>
      ) : (
        <div className="grid grid-cols-1 gap-6">
          {/* Latency Chart */}
          <div className="bg-slate-800 rounded-lg border border-slate-700 p-4">
            <h3 className="text-sm font-medium mb-4">Latency Percentiles (ms)</h3>
            {latency.length === 0 ? (
              <div className="h-64 flex items-center justify-center text-slate-500 text-sm">No latency data</div>
            ) : (
              <ResponsiveContainer width="100%" height={280}>
                <LineChart data={latency}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
                  <XAxis dataKey="time" tick={{ fill: '#94a3b8', fontSize: 11 }} />
                  <YAxis tick={{ fill: '#94a3b8', fontSize: 11 }} />
                  <Tooltip
                    contentStyle={{ background: '#1e293b', border: '1px solid #475569', borderRadius: 6, fontSize: 12 }}
                    labelStyle={{ color: '#94a3b8' }}
                  />
                  <Legend wrapperStyle={{ fontSize: 12 }} />
                  <Line type="monotone" dataKey="p50" stroke="#22d3ee" strokeWidth={1.5} dot={false} name="P50" />
                  <Line type="monotone" dataKey="p90" stroke="#a78bfa" strokeWidth={1.5} dot={false} name="P90" />
                  <Line type="monotone" dataKey="p99" stroke="#f97316" strokeWidth={2} dot={false} name="P99" />
                  <Line type="monotone" dataKey="max" stroke="#f87171" strokeWidth={1} dot={false} name="Max" strokeDasharray="4 2" />
                </LineChart>
              </ResponsiveContainer>
            )}
          </div>

          {/* Throughput Chart */}
          <div className="bg-slate-800 rounded-lg border border-slate-700 p-4">
            <h3 className="text-sm font-medium mb-4">Throughput (requests/interval)</h3>
            {throughput.length === 0 ? (
              <div className="h-64 flex items-center justify-center text-slate-500 text-sm">No throughput data</div>
            ) : (
              <ResponsiveContainer width="100%" height={280}>
                <AreaChart data={throughput}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
                  <XAxis dataKey="time" tick={{ fill: '#94a3b8', fontSize: 11 }} />
                  <YAxis tick={{ fill: '#94a3b8', fontSize: 11 }} />
                  <Tooltip
                    contentStyle={{ background: '#1e293b', border: '1px solid #475569', borderRadius: 6, fontSize: 12 }}
                    labelStyle={{ color: '#94a3b8' }}
                  />
                  <Legend wrapperStyle={{ fontSize: 12 }} />
                  <Area type="monotone" dataKey="total" fill="#8b5cf6" fillOpacity={0.15} stroke="#8b5cf6" strokeWidth={1.5} name="Total" />
                  <Area type="monotone" dataKey="errors" fill="#ef4444" fillOpacity={0.2} stroke="#ef4444" strokeWidth={1.5} name="Errors" />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </div>

          {/* Error Rate Chart */}
          <div className="bg-slate-800 rounded-lg border border-slate-700 p-4">
            <h3 className="text-sm font-medium mb-4">Error Rate (%)</h3>
            {throughput.length === 0 ? (
              <div className="h-64 flex items-center justify-center text-slate-500 text-sm">No error rate data</div>
            ) : (
              <ResponsiveContainer width="100%" height={200}>
                <AreaChart data={throughput}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
                  <XAxis dataKey="time" tick={{ fill: '#94a3b8', fontSize: 11 }} />
                  <YAxis tick={{ fill: '#94a3b8', fontSize: 11 }} domain={[0, 'auto']} />
                  <Tooltip
                    contentStyle={{ background: '#1e293b', border: '1px solid #475569', borderRadius: 6, fontSize: 12 }}
                    labelStyle={{ color: '#94a3b8' }}
                    formatter={(value) => `${Number(value).toFixed(2)}%`}
                  />
                  <Area type="monotone" dataKey="errorRate" fill="#ef4444" fillOpacity={0.2} stroke="#ef4444" strokeWidth={1.5} name="Error Rate" />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

function formatLatencyPoint(p: LatencyPoint) {
  return {
    time: new Date(p.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    p50: p.p50,
    p90: p.p90,
    p99: p.p99,
    max: p.max,
  }
}

function formatThroughputPoint(p: ThroughputPoint) {
  return {
    time: new Date(p.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    total: p.total,
    errors: p.errors,
    errorRate: p.error_rate * 100,
  }
}
