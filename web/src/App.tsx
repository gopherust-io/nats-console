import { Navigate, Route, Routes } from "react-router-dom";
import Layout from "./components/Layout";
import { AuthProvider, useAuth } from "./lib/auth";
import { ClusterProvider } from "./lib/cluster";
import AuditPage from "./pages/AuditPage";
import ClustersPage from "./pages/ClustersPage";
import ConsumerDetailPage from "./pages/ConsumerDetailPage";
import DashboardPage from "./pages/DashboardPage";
import KVBucketsPage from "./pages/KVBucketsPage";
import KVBucketPage from "./pages/KVBucketPage";
import KVKeyPage from "./pages/KVKeyPage";
import LiveStreamPage from "./pages/LiveStreamPage";
import LoginPage from "./pages/LoginPage";
import ObjectBucketPage from "./pages/ObjectBucketPage";
import ObjectBucketsPage from "./pages/ObjectBucketsPage";
import StreamDetailPage from "./pages/StreamDetailPage";
import StreamsPage from "./pages/StreamsPage";
import UsersPage from "./pages/UsersPage";

function RequireAuth({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  if (loading) {
    return <div className="login-page">Loading…</div>;
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
          <Route index element={<DashboardPage />} />
          <Route path="clusters" element={<ClustersPage />} />
          <Route path="streams" element={<StreamsPage />} />
          <Route path="streams/:name" element={<StreamDetailPage />} />
          <Route path="streams/:name/live" element={<LiveStreamPage />} />
          <Route path="streams/:name/consumers/:consumer" element={<ConsumerDetailPage />} />
          <Route path="kv" element={<KVBucketsPage />} />
          <Route path="kv/:bucket" element={<KVBucketPage />} />
          <Route path="kv/:bucket/:key" element={<KVKeyPage />} />
          <Route path="objects" element={<ObjectBucketsPage />} />
          <Route path="objects/:bucket" element={<ObjectBucketPage />} />
          <Route
            path="audit"
            element={
              <RequireAdmin>
                <AuditPage />
              </RequireAdmin>
            }
          />
          <Route
            path="users"
            element={
              <RequireAdmin>
                <UsersPage />
              </RequireAdmin>
            }
          />
        </Route>
      </Routes>
    </AuthProvider>
  );
}
