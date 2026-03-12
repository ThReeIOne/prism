import { Suspense, lazy } from 'react'
import { Routes, Route, NavLink } from 'react-router-dom'

const Dashboard = lazy(() => import('./pages/Dashboard'))
const TraceSearch = lazy(() => import('./pages/TraceSearch'))
const TraceDetail = lazy(() => import('./pages/TraceDetail'))
const Topology = lazy(() => import('./pages/Topology'))
const Stats = lazy(() => import('./pages/Stats'))

const navItems = [
  { to: '/', label: 'Dashboard' },
  { to: '/traces', label: 'Traces' },
  { to: '/topology', label: 'Topology' },
  { to: '/stats', label: 'Statistics' },
]

function PageLoader() {
  return <div className="p-6 text-slate-500">Loading...</div>
}

export default function App() {
  return (
    <div className="flex h-screen">
      {/* Sidebar */}
      <nav className="w-56 shrink-0 bg-slate-900 border-r border-slate-700 flex flex-col">
        <div className="p-5 border-b border-slate-700">
          <h1 className="text-xl font-bold tracking-wide">
            <span className="text-violet-400">P</span>rism
          </h1>
          <p className="text-xs text-slate-500 mt-1">Distributed Tracing</p>
        </div>
        <div className="flex-1 py-4">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              className={({ isActive }) =>
                `block px-5 py-2.5 text-sm transition-colors ${
                  isActive
                    ? 'text-violet-400 bg-slate-800 border-r-2 border-violet-400'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'
                }`
              }
            >
              {item.label}
            </NavLink>
          ))}
        </div>
        <div className="p-4 border-t border-slate-700 text-xs text-slate-600">
          v0.1.0
        </div>
      </nav>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        <Suspense fallback={<PageLoader />}>
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/traces" element={<TraceSearch />} />
            <Route path="/traces/:traceId" element={<TraceDetail />} />
            <Route path="/topology" element={<Topology />} />
            <Route path="/stats" element={<Stats />} />
          </Routes>
        </Suspense>
      </main>
    </div>
  )
}
