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
  configured?: boolean;
  capability?: string;
  lastCheckedAt?: string;
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

export interface DashboardIncidentWindow {
  since: string;
  until: string;
  source: string;
  confidence: number;
}

export interface DashboardEvidence {
  icon: string;
  label: string;
  value: string;
  count?: number;
  link?: string;
}

export interface DashboardLogPattern {
  label: string;
  pattern: string;
  source: string;
  severity: string;
  count?: number;
}

export interface DashboardRCA {
  summary: string;
  confidence: number;
  signal: string;
  risk: string;
  recommendedAction?: string;
  evidenceUsed?: string[];
  validatedClaims?: string[];
  unvalidatedClaims?: string[];
  negativeFeedback?: string;
}

export interface DashboardWorkloadPolicy {
  mode: string;
  autoFix: boolean;
  approvalRequired: boolean;
  availabilitySlo?: number;
  burnRateThreshold?: number;
  status: string;
}

export interface DashboardWorkloadCost {
  monthlyCost?: number;
  requestedMonthlyCost?: number;
  pricingSource?: string;
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
  helm?: DashboardHelmMapping;
}

export interface DashboardHelmMapping {
  releaseName?: string;
  namespace?: string;
  chart?: string;
  chartVersion?: string;
  repoUrl?: string;
  valueFiles?: string[];
  valuesFrom?: string[];
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
  patchTarget?: string;
  reason?: string;
  avoided?: string;
  summary?: string[];
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
  window?: DashboardIncidentWindow;
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
  logPatterns?: DashboardLogPattern[];
  rca?: DashboardRCA;
  policyState?: DashboardWorkloadPolicy;
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
  failureFingerprint?: string;
  age: string;
  workload: DashboardWorkload;
  gitops?: DashboardGitOpsMapping;
  revertPrUrl?: string;
  timeline?: DashboardTimelineStep[];
}

export interface DashboardTimelineStep {
  id: string;
  label: string;
  status: string;
  detail?: string;
  age?: string;
  url?: string;
  current?: boolean;
}

export interface DashboardGitOpsSource {
  id: string;
  controller: string;
  controllerNamespace?: string;
  controllerUrl?: string;
  app: string;
  namespace?: string;
  repo: string;
  revision: string;
  reconciledRevision?: string;
  path: string;
  manifestType: string;
  overlay?: string;
  healthStatus?: string;
  syncStatus?: string;
  operationStatus?: string;
  driftStatus?: string;
  lastSyncAt?: string;
  helm?: DashboardHelmMapping;
  workloads: number;
  workloadRefs?: DashboardWorkload[];
}

export interface DashboardWorkloadView {
  id: string;
  cluster?: string;
  workload: DashboardWorkload;
  helm?: DashboardHelmMapping;
  children?: DashboardWorkload[];
  health: string;
  status: string;
  desired?: number;
  ready?: number;
  incidents: number;
  remediations: number;
  predictions: number;
  latestIncidentId?: string;
  activeRemediationId?: number;
  gitops?: DashboardGitOpsMapping;
  evidence?: DashboardEvidence[];
  logPatterns?: DashboardLogPattern[];
  rca?: DashboardRCA;
  policyState?: DashboardWorkloadPolicy;
  cost?: DashboardWorkloadCost;
  pods?: string[];
  services?: string[];
  ingresses?: string[];
  nodes?: string[];
}

export interface DashboardWidget {
  id: string;
  title: string;
  type: string;
  scope: string;
  status: string;
  data?: Record<string, unknown>;
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
  zone?: string;
  instanceType: string;
  monthlyCost: number;
  requestedMonthlyCost?: number;
  cpuRequestedCores?: number;
  memoryRequestedMiB?: number;
  pods?: number;
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
  canInclude?: boolean;
  includeReason?: string;
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
  detail?: string;
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
  workloads?: DashboardWorkloadView[];
  incidents: DashboardIncident[];
  remediations: DashboardRemediation[];
  gitopsSources: DashboardGitOpsSource[];
  predictions: DashboardPrediction[];
  auditEvents: DashboardAuditEvent[];
  pipeline: DashboardPipelineStage[];
  widgets?: DashboardWidget[];
  settingsSections: DashboardSettingsSection[];
  clusterCostMo?: number;
  activeNodes?: number;
  nodeCosts?: DashboardNodeCost[];
}
