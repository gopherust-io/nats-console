import { lazy, Suspense } from "react";
import { Navigate, Route, Routes, useLocation, useParams } from "react-router";
import ConsolShell from "./components/ConsolShell";
import PageLoader from "./components/ui/PageLoader";
import { AccountProvider } from "./lib/account";
import { AuthProvider, useAuth } from "./lib/auth";
import { ClusterProvider, useCluster } from "./lib/cluster";
import LoginPage from "./pages/LoginPage";
import InviteAcceptPage from "./pages/InviteAcceptPage";

const AllStreamsPage = lazy(() => import("./pages/AllStreamsPage"));
const SystemsPage = lazy(() => import("./pages/SystemsPage"));
const SystemUsagePage = lazy(() => import("./pages/SystemsPage").then((m) => ({ default: m.SystemUsagePage })));
const SystemAccountsPage = lazy(() => import("./pages/SystemAccountsPage"));
const AccessPage = lazy(() => import("./pages/AccessPage"));
const AccountOverviewPage = lazy(() => import("./pages/AccountOverviewPage"));
const ConnectionsPage = lazy(() => import("./pages/ConnectionsPage"));
const ReplicasPage = lazy(() => import("./pages/ReplicasPage"));
const JetStreamHubPage = lazy(() => import("./pages/JetStreamHubPage"));
const NatsUsersPage = lazy(() => import("./pages/NatsUsersPage"));
const SharingPage = lazy(() => import("./pages/SharingPage"));
const ClustersPage = lazy(() => import("./pages/ClustersPage"));
const TopologyPage = lazy(() => import("./pages/TopologyPage"));
const DocsPage = lazy(() => import("./pages/DocsPage"));
const EventCatalogPage = lazy(() => import("./pages/EventCatalogPage"));
const EventWikipediaPage = lazy(() => import("./pages/EventWikipediaPage"));
const LiveArchitecturePage = lazy(() => import("./pages/LiveArchitecturePage"));
const ArchitectureReviewPage = lazy(() => import("./pages/ArchitectureReviewPage"));
const HiddenBottlenecksPage = lazy(() => import("./pages/HiddenBottlenecksPage"));
const ArchitectureGeneratorPage = lazy(() => import("./pages/ArchitectureGeneratorPage"));
const ArchitectureRefactorPage = lazy(() => import("./pages/ArchitectureRefactorPage"));
const ArchitectureScorePage = lazy(() => import("./pages/ArchitectureScorePage"));
const ChaosStoryPage = lazy(() => import("./pages/ChaosStoryPage"));
const StreamDetailPage = lazy(() => import("./pages/StreamDetailPage"));
const LiveStreamPage = lazy(() => import("./pages/LiveStreamPage"));
const ConsumerDetailPage = lazy(() => import("./pages/ConsumerDetailPage"));
const KVBucketsPage = lazy(() => import("./pages/KVBucketsPage"));
const KVBucketPage = lazy(() => import("./pages/KVBucketPage"));
const KVKeyPage = lazy(() => import("./pages/KVKeyPage"));
const ObjectBucketsPage = lazy(() => import("./pages/ObjectBucketsPage"));
const ObjectBucketPage = lazy(() => import("./pages/ObjectBucketPage"));
const AuditPage = lazy(() => import("./pages/AuditPage"));
const UsersPage = lazy(() => import("./pages/UsersPage"));
const AlertsPage = lazy(() => import("./pages/AlertsPage"));
const AlertRulesPage = lazy(() => import("./pages/AlertRulesPage"));

function PageLoaderFallback() {
  return <PageLoader />;
}

function SuspensePage({ children }: { children: React.ReactNode }) {
  return <Suspense fallback={<PageLoaderFallback />}>{children}</Suspense>;
}

function RequireAuth({ children }: { children: React.ReactNode }) {
  const { user, loading, sessionError } = useAuth();
  if (loading) return <PageLoaderFallback />;
  if (!user && sessionError) return <PageLoaderFallback />;
  if (!user) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

function RequireCanViewAudit({ children }: { children: React.ReactNode }) {
  const { canViewAudit, loading } = useAuth();
  if (loading) return null;
  if (!canViewAudit) return <Navigate to="/systems" replace />;
  return <>{children}</>;
}

function RequireCanManageUsers({ children }: { children: React.ReactNode }) {
  const { canManageUsers, loading } = useAuth();
  if (loading) return null;
  if (!canManageUsers) return <Navigate to="/systems" replace />;
  return <>{children}</>;
}

function RequireCanManageAlertRules({ children }: { children: React.ReactNode }) {
  const { canManageAlertRules, loading } = useAuth();
  if (loading) return null;
  if (!canManageAlertRules) return <Navigate to="/admin/alerts" replace />;
  return <>{children}</>;
}

function RedirectLegacyStream() {
  const { name } = useParams();
  const { clusterId } = useCluster();
  const location = useLocation();
  if (!clusterId || !name) return <Navigate to="/systems" replace />;
  return (
    <Navigate
      to={`/systems/${clusterId}/accounts/Default/jetstream/streams/${encodeURIComponent(name)}`}
      state={location.state}
      replace
    />
  );
}

export default function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/invite/:token" element={<InviteAcceptPage />} />
        <Route
          path="/"
          element={
            <RequireAuth>
              <ClusterProvider>
                <AccountProvider>
                  <ConsolShell />
                </AccountProvider>
              </ClusterProvider>
            </RequireAuth>
          }
        >
          <Route index element={<Navigate to="/systems" replace />} />
          <Route path="systems" element={<SuspensePage><SystemsPage /></SuspensePage>} />
          <Route path="systems/streams" element={<SuspensePage><AllStreamsPage /></SuspensePage>} />
          <Route path="systems/clusters" element={<SuspensePage><ClustersPage /></SuspensePage>} />
          <Route path="systems/:clusterId" element={<SuspensePage><SystemAccountsPage /></SuspensePage>} />
          <Route path="systems/:clusterId/usage" element={<SuspensePage><SystemUsagePage /></SuspensePage>} />
          <Route path="systems/:clusterId/replicas" element={<SuspensePage><ReplicasPage /></SuspensePage>} />
          <Route path="systems/:clusterId/access" element={<SuspensePage><AccessPage scope="system" /></SuspensePage>} />
          <Route path="systems/:clusterId/accounts/:accountName" element={<SuspensePage><AccountOverviewPage /></SuspensePage>} />
          <Route path="systems/:clusterId/accounts/:accountName/connections" element={<SuspensePage><ConnectionsPage /></SuspensePage>} />
          <Route path="systems/:clusterId/accounts/:accountName/jetstream" element={<SuspensePage><JetStreamHubPage /></SuspensePage>} />
          <Route path="systems/:clusterId/accounts/:accountName/jetstream/streams/:name" element={<SuspensePage><StreamDetailPage /></SuspensePage>} />
          <Route path="systems/:clusterId/accounts/:accountName/jetstream/streams/:name/live" element={<SuspensePage><LiveStreamPage /></SuspensePage>} />
          <Route path="systems/:clusterId/accounts/:accountName/jetstream/streams/:name/consumers/:consumer" element={<SuspensePage><ConsumerDetailPage /></SuspensePage>} />
          <Route path="systems/:clusterId/accounts/:accountName/jetstream/kv" element={<SuspensePage><KVBucketsPage /></SuspensePage>} />
          <Route path="systems/:clusterId/accounts/:accountName/jetstream/kv/:bucket" element={<SuspensePage><KVBucketPage /></SuspensePage>} />
          <Route path="systems/:clusterId/accounts/:accountName/jetstream/kv/:bucket/:key" element={<SuspensePage><KVKeyPage /></SuspensePage>} />
          <Route path="systems/:clusterId/accounts/:accountName/jetstream/objects" element={<SuspensePage><ObjectBucketsPage /></SuspensePage>} />
          <Route path="systems/:clusterId/accounts/:accountName/jetstream/objects/:bucket" element={<SuspensePage><ObjectBucketPage /></SuspensePage>} />
          <Route path="systems/:clusterId/accounts/:accountName/users" element={<SuspensePage><NatsUsersPage /></SuspensePage>} />
          <Route path="systems/:clusterId/accounts/:accountName/access" element={<SuspensePage><AccessPage scope="account" /></SuspensePage>} />
          <Route path="systems/:clusterId/accounts/:accountName/sharing" element={<SuspensePage><SharingPage /></SuspensePage>} />
          <Route
            path="systems/:clusterId/accounts/:accountName/settings"
            element={<Navigate to=".." relative="path" replace />}
          />

          <Route path="docs" element={<SuspensePage><DocsPage /></SuspensePage>} />
          <Route path="docs/event-catalog" element={<SuspensePage><EventCatalogPage /></SuspensePage>} />
          <Route path="docs/event-wikipedia" element={<SuspensePage><EventWikipediaPage /></SuspensePage>} />
          <Route path="docs/live-architecture" element={<SuspensePage><LiveArchitecturePage /></SuspensePage>} />
          <Route path="docs/architecture-review" element={<SuspensePage><ArchitectureReviewPage /></SuspensePage>} />
          <Route path="docs/hidden-bottlenecks" element={<SuspensePage><HiddenBottlenecksPage /></SuspensePage>} />
          <Route path="docs/chaos-story" element={<SuspensePage><ChaosStoryPage /></SuspensePage>} />
          <Route path="docs/architecture-generator" element={<SuspensePage><ArchitectureGeneratorPage /></SuspensePage>} />
          <Route path="docs/architecture-refactor" element={<SuspensePage><ArchitectureRefactorPage /></SuspensePage>} />
          <Route path="docs/architecture-score" element={<SuspensePage><ArchitectureScorePage /></SuspensePage>} />

          <Route path="admin/topology" element={<SuspensePage><TopologyPage /></SuspensePage>} />
          <Route path="admin/audit" element={<RequireCanViewAudit><SuspensePage><AuditPage /></SuspensePage></RequireCanViewAudit>} />
          <Route path="admin/users" element={<RequireCanManageUsers><SuspensePage><UsersPage /></SuspensePage></RequireCanManageUsers>} />
          <Route path="admin/alerts" element={<SuspensePage><AlertsPage /></SuspensePage>} />
          <Route path="admin/alert-rules" element={<RequireCanManageAlertRules><SuspensePage><AlertRulesPage /></SuspensePage></RequireCanManageAlertRules>} />

          {/* Legacy redirects keep bookmarks working */}
          <Route path="admin/clusters" element={<Navigate to="/systems/clusters" replace />} />
          <Route path="clusters" element={<Navigate to="/systems/clusters" replace />} />
          <Route path="dashboard" element={<Navigate to="/systems" replace />} />
          <Route path="streams" element={<Navigate to="/systems" replace />} />
          <Route path="streams/:name" element={<RedirectLegacyStream />} />
          <Route path="kv" element={<Navigate to="/systems" replace />} />
          <Route path="objects" element={<Navigate to="/systems" replace />} />
          <Route path="topology" element={<Navigate to="/admin/topology" replace />} />
          <Route path="resolver" element={<Navigate to="/systems" replace />} />
          <Route path="admin/resolver" element={<Navigate to="/systems" replace />} />
          <Route path="audit" element={<Navigate to="/admin/audit" replace />} />
          <Route path="users" element={<Navigate to="/admin/users" replace />} />
          <Route path="supercluster" element={<Navigate to="/admin/topology" replace />} />
          <Route path="admin/supercluster" element={<Navigate to="/admin/topology" replace />} />
          <Route path="admin/event-catalog" element={<Navigate to="/docs/event-catalog" replace />} />
        </Route>
      </Routes>
    </AuthProvider>
  );
}
