import { Navigate, Route, Routes } from "react-router-dom";
import Layout from "./components/Layout";
import { ClusterProvider } from "./lib/cluster";
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
import { getAuthHeader } from "./lib/api";

function RequireAuth({ children }: { children: React.ReactNode }) {
  if (!getAuthHeader()) {
    return <Navigate to="/login" replace />;
  }
  return <>{children}</>;
}

export default function App() {
  return (
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
      </Route>
    </Routes>
  );
}
