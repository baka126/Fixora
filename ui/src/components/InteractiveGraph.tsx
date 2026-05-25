import { useState, useMemo } from 'react';
import {
  Background,
  Controls,
  MarkerType,
  MiniMap,
  Panel,
  Position,
  ReactFlow,
  type Edge,
  type Node,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import {
  Activity,
  Boxes,
  Code2,
  Container,
  FileCode2,
  Layers3,
  Maximize2,
  Network,
  ShieldCheck,
  Zap,
  X,
} from 'lucide-react';
import type {
  DashboardDependencyEdge,
  DashboardDependencyNode,
  DashboardIncident,
} from '../types';

interface InteractiveGraphProps {
  incident: DashboardIncident | null;
}

type GraphState = {
  selectedNodeId: string | null;
  collapsedNodeIds: Set<string>;
  expanded: boolean;
};

export const InteractiveGraph = ({ incident }: InteractiveGraphProps) => {
  const nodes = useMemo(() => incident?.graph || [], [incident?.graph]);
  const edges = useMemo(() => incident?.edges || [], [incident?.edges]);
  const [state, setState] = useState<GraphState>({
    selectedNodeId: null,
    collapsedNodeIds: new Set(),
    expanded: false,
  });
  const [filterKind, setFilterKind] = useState('all');

  // Reset selected node if it's no longer in the incident data
  const effectiveSelectedId = useMemo(() => {
    if (state.selectedNodeId && nodes.some(n => n.id === state.selectedNodeId)) {
      return state.selectedNodeId;
    }
    return preferredNodeId(nodes);
  }, [nodes, state.selectedNodeId]);

  const visibleNodeIds = useMemo(() => getVisibleNodeIds(nodes, edges, state.collapsedNodeIds), [nodes, edges, state.collapsedNodeIds]);
  const graphKinds = useMemo(() => Array.from(new Set(nodes.map(getNormalizedKind).filter(Boolean))).sort(), [nodes]);

  const filteredNodes = useMemo(() => 
    nodes.filter(n => visibleNodeIds.has(n.id) && (filterKind === 'all' || getNormalizedKind(n) === filterKind)),
    [nodes, visibleNodeIds, filterKind]
  );

  const filteredEdges = useMemo(() => 
    edges.filter(([from, to]) => 
      visibleNodeIds.has(from) && 
      visibleNodeIds.has(to) && 
      !state.collapsedNodeIds.has(from) &&
      (filterKind === 'all' || (getNormalizedKindById(nodes, from) === filterKind && getNormalizedKindById(nodes, to) === filterKind))
    ),
    [edges, visibleNodeIds, state.collapsedNodeIds, nodes, filterKind]
  );

  const selectedNode = useMemo(() => nodes.find(n => n.id === effectiveSelectedId) || null, [nodes, effectiveSelectedId]);

  const toggleCollapse = (id: string) => {
    setState(prev => {
      const next = new Set(prev.collapsedNodeIds);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return { ...prev, collapsedNodeIds: next };
    });
  };

  const layoutedNodes = useMemo(() => computeLayout(filteredNodes, filteredEdges), [filteredNodes, filteredEdges]);

  const flowNodes = useMemo(() => layoutedNodes.map<Node>(n => ({
    id: n.id,
    type: 'default',
    position: { x: n.x, y: n.y },
    data: { label: <NodeLabel node={n} /> },
    style: getNodeStyle(n, n.id === effectiveSelectedId, state.expanded),
    sourcePosition: Position.Bottom,
    targetPosition: Position.Top,
  })), [layoutedNodes, effectiveSelectedId, state.expanded]);

  const flowEdges = useMemo(() => filteredEdges.map<Edge>(([from, to]) => {
    const rps = parseRPS(nodes.find(n => n.id === from)?.detail || '');
    return {
      id: `${from}-${to}`,
      source: from,
      target: to,
      type: 'smoothstep',
      animated: rps > 0,
      markerEnd: { type: MarkerType.ArrowClosed, color: '#94a3b8' },
      style: { 
        stroke: rps > 0 ? '#16a34a' : '#94a3b8', 
        strokeWidth: rps > 0 ? Math.min(1.5 + rps / 10, 4) : 1.5,
        opacity: rps > 0 ? 0.8 : 0.5 
      },
      label: rps > 0 ? `${rps.toFixed(1)} rps` : undefined,
      labelStyle: { fontSize: 10, fill: '#16a34a', fontWeight: 600 },
    };
  }), [filteredEdges, nodes]);

  if (!incident || !nodes.length) {
    return (
      <div className="rounded-lg border border-[#e5e7eb] bg-white p-6 text-center shadow-sm">
        <Network className="mx-auto h-12 w-12 text-[#94a3b8] opacity-20" />
        <h3 className="mt-4 text-[14px] font-semibold text-[#111827]">No graph data</h3>
        <p className="mt-1 text-[13px] text-[#647084]">Dependency mapping is unavailable for this workload.</p>
      </div>
    );
  }

  return (
    <section className={`rounded-lg border border-[#e5e7eb] bg-white p-3 shadow-sm transition-all ${state.expanded ? 'fixed inset-4 z-50 overflow-hidden' : 'relative'}`}>
      <header className="mb-3 flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <Activity className="h-4 w-4 text-[#2563eb]" />
          <h2 className="text-[14px] font-semibold text-[#111827]">Interactive Dependency & Traffic</h2>
        </div>
        
        <div className="flex items-center gap-2">
          {graphKinds.length > 1 && (
            <select 
              value={filterKind} 
              onChange={e => setFilterKind(e.target.value)}
              className="h-8 rounded-md border border-[#e5e7eb] bg-white px-2 text-[11px] font-medium text-[#374151] outline-none"
            >
              <option value="all">All Resources</option>
              {graphKinds.map(k => <option key={k} value={k}>{k}</option>)}
            </select>
          )}
          <button
            onClick={() => setState(p => ({ ...p, expanded: !p.expanded }))}
            className="grid h-8 w-8 place-items-center rounded-md border border-[#e5e7eb] bg-white text-[#374151] hover:bg-[#f8fafc]"
          >
            {state.expanded ? <X className="h-4 w-4" /> : <Maximize2 className="h-4 w-4" />}
          </button>
        </div>
      </header>

      <div className={`grid gap-3 ${state.expanded ? 'h-[calc(100%-48px)] grid-cols-1 xl:grid-cols-[1fr_320px]' : 'grid-cols-1'}`}>
        <div className={`${state.expanded ? 'h-full' : 'h-[320px]'} overflow-hidden rounded-md border border-[#e5e7eb] bg-[#fbfdff]`}>
          <ReactFlow
            nodes={flowNodes}
            edges={flowEdges}
            onNodeClick={(_, n) => setState(p => ({ ...p, selectedNodeId: n.id }))}
            fitView
            fitViewOptions={{ padding: 0.2 }}
            proOptions={{ hideAttribution: true }}
          >
            <Background color="#dbe3ef" gap={20} />
            <Controls showInteractive={false} />
            {state.expanded && (
              <MiniMap 
                nodeColor={n => getKindColor(nodes.find(item => item.id === n.id)?.label || '')}
                maskColor="rgba(241,245,249,0.7)"
              />
            )}
            <Panel position="top-left" className="rounded-md border border-[#e5e7eb] bg-white/80 p-2 text-[10px] font-medium text-[#647084] backdrop-blur-sm">
              <div className="flex flex-col gap-1">
                <div className="flex items-center gap-1.5"><span className="h-1.5 w-1.5 rounded-full bg-[#16a34a]" /> Animated flow indicates live RPS</div>
                <div className="flex items-center gap-1.5"><span className="h-1.5 w-1.5 rounded-full bg-[#2563eb]" /> Blue indicates active workload path</div>
              </div>
            </Panel>
          </ReactFlow>
        </div>

        <aside className={`${state.expanded ? 'block min-h-0 overflow-y-auto p-1' : 'hidden xl:block'}`}>
          <NodeDetailPanel 
            node={selectedNode} 
            incident={incident} 
            onToggleCollapse={() => selectedNode && toggleCollapse(selectedNode.id)}
            isCollapsed={!!selectedNode && state.collapsedNodeIds.has(selectedNode.id)}
          />
        </aside>
      </div>
      
      {!state.expanded && selectedNode && (
        <div className="mt-3 flex items-center justify-between border-t border-[#f3f4f6] pt-3 text-[11px] text-[#647084]">
          <div className="flex items-center gap-2">
            <Zap className="h-3 w-3 text-[#16a34a]" />
            <span>Traffic nodes show real-time flow metrics</span>
          </div>
          <button 
            onClick={() => setState(p => ({ ...p, expanded: true }))}
            className="font-semibold text-[#2563eb] hover:underline"
          >
            View Full Topology
          </button>
        </div>
      )}
    </section>
  );
};

const NodeLabel = ({ node }: { node: DashboardDependencyNode }) => {
  const isTraffic = node.label.toLowerCase().includes('traffic');
  const color = getKindColor(node.label);
  return (
    <div className="flex min-w-0 items-center gap-2 text-left text-[10px] leading-tight">
      <span className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-white shadow-sm" style={{ backgroundColor: color }}>
        {isTraffic ? <Zap className="h-3.5 w-3.5" /> : getGlyph(node.label, 'h-4 w-4')}
      </span>
      <div className="min-w-0 flex-1">
        <div className="truncate font-bold text-[#111827]">{node.label}</div>
        <div className="truncate text-[#475569] opacity-80">{node.detail}</div>
      </div>
    </div>
  );
};

const NodeDetailPanel = ({ node, incident, onToggleCollapse, isCollapsed }: { node: DashboardDependencyNode | null, incident: DashboardIncident, onToggleCollapse: () => void, isCollapsed: boolean }) => {
  if (!node) return <div className="rounded-md border border-dashed border-[#e5e7eb] p-6 text-center text-[12px] text-[#647084]">Select a node to inspect resource context.</div>;

  const rps = parseRPS(node.detail);
  const isTraffic = node.label.toLowerCase().includes('traffic');

  return (
    <div className="rounded-lg border border-[#e5e7eb] bg-white p-4 shadow-sm">
      <header className="flex items-start gap-3">
        <span className="grid h-10 w-10 shrink-0 place-items-center rounded-lg text-white shadow-md" style={{ backgroundColor: getKindColor(node.label) }}>
          {isTraffic ? <Zap className="h-5 w-5" /> : getGlyph(node.label, 'h-5 w-5')}
        </span>
        <div className="min-w-0">
          <h3 className="truncate text-[15px] font-bold text-[#111827]">{node.label}</h3>
          <p className="mt-0.5 truncate text-[12px] text-[#647084]">{node.detail}</p>
        </div>
      </header>

      <div className="mt-4 space-y-3 border-t border-[#f3f4f6] pt-4">
        {rps > 0 && (
          <div className="flex items-center gap-2 rounded-md bg-[#f0fdf4] p-2 text-[#166534]">
            <Activity className="h-4 w-4" />
            <span className="text-[13px] font-semibold">{rps.toFixed(2)} Requests Per Second</span>
          </div>
        )}
        
        <div className="grid grid-cols-2 gap-4 text-[12px]">
          <div>
            <div className="font-semibold text-[#111827]">Resource</div>
            <div className="mt-0.5 text-[#4b5563]">{node.label}</div>
          </div>
          <div>
            <div className="font-semibold text-[#111827]">Namespace</div>
            <div className="mt-0.5 text-[#4b5563]">{incident.workload.namespace}</div>
          </div>
          <div>
            <div className="font-semibold text-[#111827]">Status</div>
            <div className="mt-0.5 text-[#4b5563]">{incident.status || 'Active'}</div>
          </div>
          <div>
            <div className="font-semibold text-[#111827]">Confidence</div>
            <div className="mt-0.5 text-[#4b5563]">{incident.confidence}%</div>
          </div>
        </div>

        {incident.gitops && (
          <div className="rounded-md bg-[#f8fafc] p-2 text-[11px]">
            <div className="font-semibold text-[#475569]">GitOps Source</div>
            <div className="mt-1 flex items-center gap-1 text-[#2563eb]">
              <FileCode2 className="h-3 w-3" />
              <span className="truncate">{incident.gitops.repo}/{incident.gitops.path}</span>
            </div>
          </div>
        )}
      </div>

      <button
        onClick={onToggleCollapse}
        className="mt-4 w-full rounded-md border border-[#e5e7eb] py-2 text-[12px] font-medium text-[#374151] hover:bg-[#f8fafc]"
      >
        {isCollapsed ? 'Expand Dependencies' : 'Collapse Dependencies'}
      </button>
    </div>
  );
};

// Helpers
const getKindColor = (kind: string) => {
  const lower = kind.toLowerCase();
  if (lower.includes('traffic')) return '#10b981';
  if (lower.includes('helm') || lower.includes('chart')) return '#2563eb';
  if (lower.includes('active') || lower.includes('deployment')) return '#2563eb';
  if (lower.includes('config') || lower.includes('secret') || lower.includes('warning')) return '#f97316';
  if (lower.includes('stateful')) return '#0ea5e9';
  if (lower.includes('service') || lower.includes('ingress')) return '#16a34a';
  if (lower.includes('pod')) return '#6366f1';
  return '#64748b';
};

const getGlyph = (kind: string, className: string) => {
  const lower = kind.toLowerCase();
  if (lower.includes('helm')) return <Boxes className={className} />;
  if (lower.includes('config')) return <FileCode2 className={className} />;
  if (lower.includes('secret')) return <ShieldCheck className={className} />;
  if (lower.includes('service')) return <Network className={className} />;
  if (lower.includes('ingress')) return <Layers3 className={className} />;
  if (lower.includes('pod')) return <Container className={className} />;
  return <Code2 className={className} />;
};

const getNormalizedKind = (n: DashboardDependencyNode) => {
  const raw = `${n.kind || ''} ${n.label || ''}`.toLowerCase();
  if (raw.includes('helm')) return 'Helm';
  if (raw.includes('traffic')) return 'Traffic';
  if (raw.includes('deployment')) return 'Deployment';
  if (raw.includes('pod')) return 'Pod';
  if (raw.includes('config')) return 'ConfigMap';
  if (raw.includes('secret')) return 'Secret';
  if (raw.includes('service')) return 'Service';
  return n.kind || n.label || 'Resource';
};

const getNormalizedKindById = (nodes: DashboardDependencyNode[], id: string) => {
  const node = nodes.find(n => n.id === id);
  return node ? getNormalizedKind(node) : 'Resource';
};

const parseRPS = (detail: string): number => {
  const match = detail.match(/(\d+\.?\d*)\s*rps/i);
  return match ? parseFloat(match[1]) : 0;
};

const getVisibleNodeIds = (nodes: DashboardDependencyNode[], edges: DashboardDependencyEdge[], collapsed: Set<string>) => {
  const visible = new Set(nodes.map(n => n.id));
  const childrenMap = new Map<string, string[]>();
  edges.forEach(([from, to]) => {
    const list = childrenMap.get(from) || [];
    list.push(to);
    childrenMap.set(from, list);
  });

  const hideDescendants = (id: string) => {
    (childrenMap.get(id) || []).forEach(childId => {
      if (visible.has(childId)) {
        visible.delete(childId);
        hideDescendants(childId);
      }
    });
  };

  collapsed.forEach(hideDescendants);
  return visible;
};

const computeLayout = (nodes: DashboardDependencyNode[], edges: DashboardDependencyEdge[]) => {
  if (!nodes.length) return [];
  const nodeIds = new Set(nodes.map(n => n.id));
  const adj = new Map<string, string[]>();
  const inDegree = new Map<string, number>();
  
  nodes.forEach(n => inDegree.set(n.id, 0));
  edges.forEach(([from, to]) => {
    if (nodeIds.has(from) && nodeIds.has(to)) {
      const list = adj.get(from) || [];
      list.push(to);
      adj.set(from, list);
      inDegree.set(to, (inDegree.get(to) || 0) + 1);
    }
  });

  const levels = new Map<string, number>();
  const queue = nodes.filter(n => (inDegree.get(n.id) || 0) === 0).map(n => n.id);
  
  if (queue.length === 0 && nodes.length > 0) queue.push(nodes[0].id);

  let i = 0;
  while (i < queue.length) {
    const id = queue[i++];
    const level = levels.get(id) || 0;
    (adj.get(id) || []).forEach(childId => {
      if (!levels.has(childId) || levels.get(childId)! < level + 1) {
        levels.set(childId, level + 1);
        queue.push(childId);
      }
    });
  }

  nodes.forEach(n => { if (!levels.has(n.id)) levels.set(n.id, 0); });

  const levelGroups = new Map<number, DashboardDependencyNode[]>();
  nodes.forEach(n => {
    const l = levels.get(n.id)!;
    const group = levelGroups.get(l) || [];
    group.push(n);
    levelGroups.set(l, group);
  });

  return nodes.map(n => {
    const l = levels.get(n.id)!;
    const group = levelGroups.get(l)!;
    const index = group.findIndex(item => item.id === n.id);
    const total = group.length;
    return {
      ...n,
      x: index * 200 - ((total - 1) * 200) / 2,
      y: l * 120,
    };
  });
};

const getNodeStyle = (node: DashboardDependencyNode, selected: boolean, expanded: boolean) => ({
  width: expanded ? 160 : 148,
  minHeight: expanded ? 58 : 54,
  padding: '8px',
  borderRadius: 10,
  border: selected ? '2px solid #2563eb' : '1px solid #dbe3ef',
  boxShadow: selected ? '0 10px 25px rgba(37,99,235,0.15)' : '0 1px 3px rgba(0,0,0,0.05)',
  background: node.label.toLowerCase().includes('active') ? '#eff6ff' : '#ffffff',
});

const preferredNodeId = (nodes: DashboardDependencyNode[]) => 
  nodes.find(n => /helm/i.test(n.label))?.id || nodes[0]?.id || null;
