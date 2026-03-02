import { lazy, Suspense } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { Toaster } from "sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { useAuth } from "@/hooks/useAuth";
import AppLayout from "@/components/layout/AppLayout";
import { Loader2 } from "lucide-react";

const Login = lazy(() => import("./pages/Login"));
const Dashboard = lazy(() => import("./pages/Dashboard"));
const ClientDetail = lazy(() => import("./pages/ClientDetail"));
const NotFound = lazy(() => import("./pages/NotFound"));

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, staleTime: 2_000 },
  },
});

function PageLoader() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <Loader2 className="w-8 h-8 animate-spin text-primary" />
    </div>
  );
}

function AuthGate({ children }: { children: React.ReactNode }) {
  const { user, isLoading } = useAuth();
  if (isLoading) return <PageLoader />;
  if (!user) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <BrowserRouter>
          <Suspense fallback={<PageLoader />}>
            <Routes>
              <Route path="/login" element={<Login />} />
              <Route
                path="/"
                element={
                  <AuthGate>
                    <AppLayout><Dashboard /></AppLayout>
                  </AuthGate>
                }
              />
              <Route
                path="/client/:machineId"
                element={
                  <AuthGate>
                    <AppLayout><ClientDetail /></AppLayout>
                  </AuthGate>
                }
              />
              <Route path="*" element={<NotFound />} />
            </Routes>
          </Suspense>
        </BrowserRouter>
        <Toaster position="top-right" richColors />
      </TooltipProvider>
    </QueryClientProvider>
  );
}

export default App;
