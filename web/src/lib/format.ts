export function formatDuration(us: number): string {
  if (us < 1000) return `${us}μs`
  if (us < 1_000_000) return `${(us / 1000).toFixed(1)}ms`
  return `${(us / 1_000_000).toFixed(2)}s`
}

export function formatTimestamp(us: number): string {
  return new Date(us / 1000).toLocaleString()
}

export function statusColor(status: string): string {
  return status === 'error' ? 'text-red-400' : 'text-emerald-400'
}

export function kindBadgeColor(kind: string): string {
  switch (kind?.toUpperCase()) {
    case 'SERVER': return 'bg-blue-500/20 text-blue-400'
    case 'CLIENT': return 'bg-amber-500/20 text-amber-400'
    case 'PRODUCER': return 'bg-purple-500/20 text-purple-400'
    case 'CONSUMER': return 'bg-cyan-500/20 text-cyan-400'
    default: return 'bg-slate-500/20 text-slate-400'
  }
}
