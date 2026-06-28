import { lazy, Suspense } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import Layout from "./components/Layout";
import PageLoader from "./components/ui/PageLoader";
import { AuthProvider, useAuth } from "./lib/auth";
import { ClusterProvider } from "./lib/cluster";
import LoginPage from "./pages/LoginPage";

const DashboardPage = lazy(() => import("./pages/DashboardPage"));
const ClustersPage = lazy(() => import("./pages/ClustersPage"));
const TopologyPage = lazy(() => import("./pages/TopologyPage"));
const SuperclusterPage = lazy(() => import("./pages/SuperclusterPage"));
const ResolverPage = lazy(() => import("./pages/ResolverPage"));
const StreamsPage = lazy(() => import("./pages/StreamsPage"));
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
const ProfilingPage = lazy(() => import("./pages/ProfilingPage"));

function PageLoaderFallback() {
  return <PageLoader />;
}

function RequireAuth({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  if (loading) {
    return <PageLoaderFallback />;
  }
  if (!user) {
    return <Navigate to="/login" replace />;
  }
  return <>{children}</>;
}

function RequireAdmin({ children }: { children: React.ReactNode }) {
  const { isAdmin, loading } = useAuth();
  if (loading) return null;
  if (!isAdmin) return <Navigate to="/" replace />;
  return <>{children}</>;
}

function RequireCanViewAudit({ children }: { children: React.ReactNode }) {
  const { canViewAudit, loading } = useAuth();
  if (loading) return null;
  if (!canViewAudit) return <Navigate to="/" replace />;
  return <>{children}</>;
}

function RequireCanManageUsers({ children }: { children: React.ReactNode }) {
  const { canManageUsers, loading } = useAuth();
  if (loading) return null;
  if (!canManageUsers) return <Navigate to="/" replace />;
  return <>{children}</>;
}

export default function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          path="/"
          element={
            <RequireAuth>
              <ClusterProvider>
                <Layout />
              </ClusterProvider>
            </RequireAuth>
          }
        >
          <Route
            index
            element={
              <Suspense fallback={<PageLoaderFallback />}>
                <DashboardPage />
              </Suspense>
            }
          />
          <Route
            path="clusters"
            element={
              <Suspense fallback={<PageLoaderFallback />}>
                <ClustersPage />
              </Suspense>
            }
          />
          <Route
            path="profiling"
            element={
              <RequireAdmin>
                <Suspense fallback={<PageLoaderFallback />}>
                  <ProfilingPage />
                </Suspense>
              </RequireAdmin>
            }
          />
          <Route
            path="topology"
            element={
              <Suspense fallback={<PageLoaderFallback />}>
                <TopologyPage />
              </Suspense>
            }
          />
          <Route
            path="supercluster"
            element={
              <Suspense fallback={<PageLoaderFallback />}>
                <SuperclusterPage />
              </Suspense>
            }
          />
          <Route
            path="resolver"
            element={
              <Suspense fallback={<PageLoaderFallback />}>
                <ResolverPage />
              </Suspense>
            }
          />
          <Route
            path="streams"
            element={
              <Suspense fallback={<PageLoaderFallback />}>
                <StreamsPage />
              </Suspense>
            }
          />
          <Route
            path="streams/:name"
            element={
              <Suspense fallback={<PageLoaderFallback />}>
                <StreamDetailPage />
              </Suspense>
            }
          />
          <Route
            path="streams/:name/live"
            element={
              <Suspense fallback={<PageLoaderFallback />}>
                <LiveStreamPage />
              </Suspense>
            }
          />
          <Route
            path="streams/:name/consumers/:consumer"
            element={
              <Suspense fallback={<PageLoaderFallback />}>
                <ConsumerDetailPage />
              </Suspense>
            }
          />
          <Route
            path="kv"
            element={
              <Suspense fallback={<PageLoaderFallback />}>
                <KVBucketsPage />
              </Suspense>
            }
          />
          <Route
            path="kv/:bucket"
            element={
              <Suspense fallback={<PageLoaderFallback />}>
                <KVBucketPage />
              </Suspense>
            }
          />
          <Route
            path="kv/:bucket/:key"
            element={
              <Suspense fallback={<PageLoaderFallback />}>
                <KVKeyPage />
              </Suspense>
            }
          />
          <Route
            path="objects"
            element={
              <Suspense fallback={<PageLoaderFallback />}>
                <ObjectBucketsPage />
              </Suspense>
            }
          />
          <Route
            path="objects/:bucket"
            element={
              <Suspense fallback={<PageLoaderFallback />}>
                <ObjectBucketPage />
              </Suspense>
            }
          />
          <Route
            path="audit"
            element={
              <RequireCanViewAudit>
                <Suspense fallback={<PageLoaderFallback />}>
                  <AuditPage />
                </Suspense>
              </RequireCanViewAudit>
            }
          />
          <Route
            path="users"
            element={
              <RequireCanManageUsers>
                <Suspense fallback={<PageLoaderFallback />}>
                  <UsersPage />
                </Suspense>
              </RequireCanManageUsers>
            }
          />
        </Route>
      </Routes>
    </AuthProvider>
  );
}
