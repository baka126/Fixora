export interface User {
  id: string;
  username: string;
  role: 'admin' | 'operator' | 'viewer';
  created_at: string;
}

export interface DashboardMetadata {
  version: string;
  buildDate: string;
  systemHealth: string;
  notifications: number;
}

export interface DashboardPolicy {
  mode: string;
  description: string;
}

export interface DashboardIntegration {
  name: string;
  status: 'ok' | 'neutral' | 'empty' | 'error' | string;
  detail?: string;
}

export interface DashboardAvailability {
  name: string;
  status: string;
  message: string;
}

export interface DashboardKPI {
  label: string;
  value: string;
  detail: string;
  icon?: string;
  tone?: 'critical' | 'warning' | 'success' | 'info' | string;
  delta?: string;
  trend?: number[];
}

export interface DashboardWorkload {
  kind: string;
  name: string;
  namespace: string;
  podName?: string;
}

export interface DashboardEvidence {
  icon: string;
  label: string;
  value: string;
  count?: number;
  link?: string;
}

export interface DashboardGitOpsMapping {
  controller: string;
  app: string;
  namespace?: string;
  repo: string;
  revision: string;
  path: string;
  manifestType: string;
  overlay?: string;
}

export interface DashboardRecommendedPR {
  branch: string;
  title: string;
  files: string[];
  fileCount: number;
  strategy: string;
  status: string;
  risk: string;
  approverRequired: boolean;
  url?: string;
}

export interface DashboardGuardrail {
  label: string;
  status: string;
  detail?: string;
}

export interface DashboardDependencyNode {
  id: string;
  label: string;
  detail: string;
  x: number;
  y: number;
  kind: string;
}

export type DashboardDependencyEdge = [string, string];

export interface DashboardIncident {
  id: string;
  cluster?: string;
  workload: DashboardWorkload;
  status: string;
  cause: string;
  confidence: number;
  source: string;
  age: string;
  severity: string;
  priority?: string;
  risk: string;
  evidence?: DashboardEvidence[];
  gitops?: DashboardGitOpsMapping;
  pr?: DashboardRecommendedPR;
  guardrails?: DashboardGuardrail[];
  graph?: DashboardDependencyNode[];
  edges?: DashboardDependencyEdge[];
}

export interface DashboardRemediation {
  id: number;
  status: string;
  title: string;
  repository: string;
  baseBranch: string;
  headBranch: string;
  prUrl?: string;
  files: string[];
  strategy: string;
  failureReason?: string;
  age: string;
  workload: DashboardWorkload;
  gitops?: DashboardGitOpsMapping;
}

export interface DashboardGitOpsSource {
  id: string;
  controller: string;
  app: string;
  namespace?: string;
  repo: string;
  revision: string;
  path: string;
  manifestType: string;
  overlay?: string;
  workloads: number;
}

export interface Group {
  id: string;
  name: string;
  description: string;
  created_at: string;
}

export interface User {
  id: string;
  username: string;
  role: 'admin' | 'operator' | 'viewer';
  groups?: Group[];
  created_at: string;
}

export interface DashboardPrediction {
  id: string;
  namespace: string;
  podName: string;
  lastAlertAge: string;
  lastGrowthRate: number;
  risk: string;
  preventionCostMo?: number;
  downtimeRiskHr?: number;
}

export interface DashboardNodeCost {
  name: string;
  vendor: string;
  region: string;
  instanceType: string;
  monthlyCost: number;
  pricingSource?: string;
  status: string;
}

export interface DashboardAlertLabel {
  key: string;
  value: string;
}

export interface DashboardActiveAlert {
  id: string;
  alertName: string;
  severity: string;
  namespace: string;
  resourceKind: string;
  resourceName: string;
  podName?: string;
  status: string;
  startsAt: string;
  age: string;
  summary?: string;
  used: boolean;
  decision: string;
  reason: string;
  labels?: DashboardAlertLabel[];
}

export interface DashboardAuditEvent {
  id: string;
  type: string;
  status: string;
  subject: string;
  detail: string;
  timestamp: string;
}

export interface InvestigationDetail {
  id: number;
  namespace: string;
  podName: string;
  timestamp: string;
  reason: string;
  metricProof: string;
  clusterContext: string;
  historicalPattern: string;
  eventTimeline: string;
  stackTrace: string;
  rootCause: string;
  aiConfidence: number;
  finopsImpact: string;
  finopsDetails: string;
  aiPrompt: string;
  aiResponse: string;
}

export interface DashboardPipelineItem {
  id: string;
  label: string;
  repository?: string;
  age: string;
  url?: string;
}

export interface DashboardPipelineStage {
  id: string;
  label: string;
  count: string;
  state: string;
  note: string;
  icon: string;
  items?: DashboardPipelineItem[];
}

export interface DashboardSettingsSection {
  name: string;
  description: string;
  status: string;
}

export interface DashboardState {
  environment: string;
  timeRange: string;
  generatedAt?: string;
  metadata?: DashboardMetadata;
  policy: DashboardPolicy;
  integrations: DashboardIntegration[];
  availability: DashboardAvailability[];
  kpis: DashboardKPI[];
  incidents: DashboardIncident[];
  remediations: DashboardRemediation[];
  gitopsSources: DashboardGitOpsSource[];
  predictions: DashboardPrediction[];
  auditEvents: DashboardAuditEvent[];
  pipeline: DashboardPipelineStage[];
  settingsSections: DashboardSettingsSection[];
  clusterCostMo?: number;
  activeNodes?: number;
  nodeCosts?: DashboardNodeCost[];
}
