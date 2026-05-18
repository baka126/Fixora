import React, { useEffect, useState } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { useStore } from './store/useStore';
import { Layout } from './components/Layout';
import { Login } from './pages/Login';
import { Setup } from './pages/Setup';
import { Dashboard } from './pages/Dashboard';
import { DataPage } from './pages/DataPage';
import { Alerts } from './pages/Alerts';
import { Workloads } from './pages/Workloads';
import { apiClient } from './api/client';
import { Loader2 } from 'lucide-react';

const ProtectedRoute = ({ children }: { children: React.ReactNode }) => {
  const token = useStore((state) => state.token);
  if (!token) {
    return <Navigate to="/login" replace />;
  }
  return <>{children}</>;
};

const SetupChecker = ({ children }: { children: React.ReactNode }) => {
  const [needsSetup, setNeedsSetup] = useState<boolean | null>(null);

  useEffect(() => {
    const checkSetup = async () => {
      try {
        const { data } = await apiClient.get('/auth/setup-status');
        setNeedsSetup(data.needsSetup);
      } catch (err) {
        console.error('Failed to check setup status', err);
        setNeedsSetup(false);
      }
    };
    checkSetup();
  }, []);

  if (needsSetup === null) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#f3f6f8]">
        <Loader2 className="w-8 h-8 animate-spin text-[#157f7d]" />
      </div>
    );
  }

  // If setup is needed but we aren't on the setup route, redirect there
  if (needsSetup && window.location.pathname !== '/setup') {
    return <Navigate to="/setup" replace />;
  }

  // If setup is NOT needed but we ARE on the setup route, redirect to login
  if (!needsSetup && window.location.pathname === '/setup') {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
};

function App() {
  return (
    <BrowserRouter>
      <SetupChecker>
        <Routes>
          <Route path="/setup" element={<Setup />} />
          <Route path="/login" element={<Login />} />
          <Route
            path="/"
            element={
              <ProtectedRoute>
                <Layout />
              </ProtectedRoute>
            }
          >
            <Route index element={<Dashboard />} />
            <Route path="workloads" element={<Workloads />} />
            <Route path="alerts" element={<Alerts />} />
            <Route path="remediations" element={<DataPage kind="remediations" />} />
            <Route path="gitops" element={<DataPage kind="gitops" />} />
            <Route path="predictions" element={<DataPage kind="predictions" />} />
            <Route path="audit" element={<DataPage kind="audit" />} />
            <Route path="settings" element={<DataPage kind="settings" />} />
            <Route path="*" element={<Dashboard />} />
          </Route>
        </Routes>
      </SetupChecker>
    </BrowserRouter>
  );
}

export default App;
