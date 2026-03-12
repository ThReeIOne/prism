import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getServices, type ServiceInfo } from '../lib/api'

export default function Dashboard() {
  const [services, setServices] = useState<ServiceInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    getServices()
      .then((res) => setServices(res.services || []))
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  return (
    <div className="p-6">
      <h2 className="text-2xl font-semibold mb-6">Dashboard</h2>

      {/* Summary cards */}
      <div className="grid grid-cols-3 gap-4 mb-8">
        <Card label="Services" value={services.length} />
        <Card
          label="Avg Error Rate"
          value={
            services.length
              ? (services.reduce((s, v) => s + v.error_rate, 0) / services.length * 100).toFixed(2) + '%'
              : '-'
          }
          warn={services.some((s) => s.error_rate > 0.05)}
        />
        <Card
          label="Max P99 Latency"
          value={
            services.length
              ? Math.max(...services.map((s) => s.p99_latency_ms)).toFixed(1) + 'ms'
              : '-'
          }
        />
      </div>

      {/* Service table */}
      <div className="bg-slate-800 rounded-lg border border-slate-700 overflow-hidden">
        <div className="px-4 py-3 border-b border-slate-700 flex items-center justify-between">
          <h3 className="font-medium">Services (last 1h)</h3>
          <Link to="/traces" className="text-sm text-violet-400 hover:text-violet-300">
            Search Traces &rarr;
          </Link>
        </div>
        {loading ? (
          <div className="p-8 text-center text-slate-500">Loading...</div>
        ) : error ? (
          <div className="p-8 text-center text-red-400">{error}</div>
        ) : services.length === 0 ? (
          <div className="p-8 text-center text-slate-500">No service data yet. Send some traces first.</div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-slate-400 text-left">
                <th className="px-4 py-2 font-medium">Service</th>
                <th className="px-4 py-2 font-medium text-right">QPS</th>
                <th className="px-4 py-2 font-medium text-right">Error Rate</th>
                <th className="px-4 py-2 font-medium text-right">P99 Latency</th>
              </tr>
            </thead>
            <tbody>
              {services.map((svc) => (
                <tr key={svc.name} className="border-t border-slate-700/50 hover:bg-slate-700/30">
                  <td className="px-4 py-2.5">
                    <Link to={`/traces?service=${svc.name}`} className="text-violet-400 hover:underline">
                      {svc.name}
                    </Link>
                  </td>
                  <td className="px-4 py-2.5 text-right font-mono">{svc.qps.toFixed(1)}</td>
                  <td className={`px-4 py-2.5 text-right font-mono ${svc.error_rate > 0.05 ? 'text-red-400' : 'text-emerald-400'}`}>
                    {(svc.error_rate * 100).toFixed(2)}%
                  </td>
                  <td className="px-4 py-2.5 text-right font-mono">{svc.p99_latency_ms.toFixed(1)}ms</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}

function Card({ label, value, warn }: { label: string; value: string | number; warn?: boolean }) {
  return (
    <div className="bg-slate-800 rounded-lg border border-slate-700 p-4">
      <p className="text-xs text-slate-500 uppercase tracking-wide">{label}</p>
      <p className={`text-2xl font-bold mt-1 ${warn ? 'text-red-400' : 'text-slate-100'}`}>{value}</p>
    </div>
  )
}
