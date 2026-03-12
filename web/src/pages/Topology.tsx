import { useEffect, useState, useCallback } from 'react'
import {
  ReactFlow,
  Background,
  Controls,
  type Node,
  type Edge,
  Position,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { getDependencies, type DepNode, type DepEdge } from '../lib/api'

export default function Topology() {
  const [nodes, setNodes] = useState<Node[]>([])
  const [edges, setEdges] = useState<Edge[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    getDependencies()
      .then((data) => {
        const depNodes = data.nodes || []
        const depEdges = data.edges || []
        setNodes(layoutNodes(depNodes, depEdges))
        setEdges(
          depEdges.map((e, i) => ({
            id: `e-${i}`,
            source: e.source,
            target: e.target,
            label: `${e.call_count} calls`,
            animated: e.error_count > 0,
            style: { stroke: e.error_count > 0 ? '#f87171' : '#8b5cf6' },
            labelStyle: { fill: '#94a3b8', fontSize: 11 },
            labelBgStyle: { fill: '#1e293b' },
            labelBgPadding: [4, 2] as [number, number],
          }))
        )
      })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  const onInit = useCallback(() => {}, [])

  if (loading) return <div className="p-6 text-slate-500">Loading...</div>
  if (error) return <div className="p-6 text-red-400">{error}</div>
  if (nodes.length === 0) return <div className="p-6 text-slate-500">No dependency data yet.</div>

  return (
    <div className="p-6 h-full flex flex-col">
      <h2 className="text-2xl font-semibold mb-4">Service Topology</h2>
      <div className="flex-1 bg-slate-800 rounded-lg border border-slate-700 overflow-hidden" style={{ minHeight: 500 }}>
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onInit={onInit}
          fitView
          proOptions={{ hideAttribution: true }}
          style={{ background: '#1e293b' }}
        >
          <Background color="#334155" gap={20} />
          <Controls
            style={{ background: '#334155', borderColor: '#475569' }}
          />
        </ReactFlow>
      </div>
    </div>
  )
}

function layoutNodes(depNodes: DepNode[], depEdges: DepEdge[]): Node[] {
  // Find root nodes (sources not appearing as targets)
  const targets = new Set(depEdges.map((e) => e.target))
  const roots = depNodes.filter((n) => !targets.has(n.id))
  if (roots.length === 0 && depNodes.length > 0) roots.push(depNodes[0])

  // BFS layered layout
  const layers: string[][] = []
  const visited = new Set<string>()
  let current = roots.map((r) => r.id)
  while (current.length > 0) {
    const layer: string[] = []
    const next: string[] = []
    for (const id of current) {
      if (visited.has(id)) continue
      visited.add(id)
      layer.push(id)
      for (const e of depEdges) {
        if (e.source === id && !visited.has(e.target)) {
          next.push(e.target)
        }
      }
    }
    if (layer.length > 0) layers.push(layer)
    current = next
  }

  // Place remaining unvisited nodes
  const remaining = depNodes.filter((n) => !visited.has(n.id)).map((n) => n.id)
  if (remaining.length > 0) layers.push(remaining)

  const xGap = 250
  const yGap = 100

  const result: Node[] = []
  for (let col = 0; col < layers.length; col++) {
    const layer = layers[col]
    const yOffset = -(layer.length - 1) * yGap / 2
    for (let row = 0; row < layer.length; row++) {
      result.push({
        id: layer[row],
        position: { x: col * xGap, y: yOffset + row * yGap },
        data: { label: layer[row] },
        sourcePosition: Position.Right,
        targetPosition: Position.Left,
        style: {
          background: '#334155',
          color: '#e2e8f0',
          border: '1px solid #6366f1',
          borderRadius: 8,
          padding: '8px 16px',
          fontSize: 13,
          fontWeight: 500,
        },
      })
    }
  }
  return result
}
