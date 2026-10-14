import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { useAuth } from "./context/AuthContext";
import { getModules } from "./lib/api";
import MoreLabsNotice from "./components/MoreLabsNotice";
import AppLayout from "./layout/AppLayout";
import LabPage from "./pages/LabPage";
import Landing from "./pages/Landing";
import LegalPage from "./pages/LegalPage";
import LeaderboardsPage from "./pages/LeaderboardsPage";
import ProfilePage from "./pages/ProfilePage";
import Login from "./pages/Login";
import AdminPage from "./pages/AdminPage";
import UsersPage from "./pages/UsersPage";
import LabAdminPage from "./pages/LabAdminPage";
import DriverAdminPage from "./pages/DriverAdminPage";
import SettingsPage from "./pages/SettingsPage";
import PoppedPane from "./pages/PoppedPane";
import NotFound from "./pages/NotFound";

function ProtectedRoute({ children }: { children: JSX.Element }) {
  const { isAuthenticated, isLoading } = useAuth();
  const location = useLocation();

  if (isLoading) return null;
  if (!isAuthenticated) {
    const redirect = encodeURIComponent(location.pathname);
    return <Navigate to={`/login?redirect=${redirect}`} replace />;
  }
  return children;
}

// UX-only gate - the actual enforcement is server-side (requireAdmin on
// every admin route/endpoint); this just avoids showing a page whose data
// would 403 anyway.
function RequireAdmin({ children }: { children: JSX.Element }) {
  const { user } = useAuth();
  if (user?.role !== "admin") return <Navigate to="/" replace />;
  return children;
}

// The public entry at "/": the marketing Landing, shown to everyone. We no
// longer bounce signed-in users to /labs - the Landing itself is auth-aware, so
// a returning member sees the same page but with a one-click "go to your labs"
// path instead of a sign-in prompt.
function Home() {
  return <Landing />;
}

// The authenticated labs home ("/labs"). This used to hard-redirect to a
// specific module slug, which broke once lab access became grant-based: a newly
// registered user has no grants at all, and a user granted some *other* lab
// would land on one they can't start. Send them to their first entitled lab
// instead, and say so plainly when there isn't one.
function LabsIndex() {
  const { data: modules, isLoading, isError } = useQuery({
    queryKey: ["modules"],
    queryFn: getModules,
  });

  if (isLoading) return null;
  if (isError) {
    return (
      <div className="p-6 text-sm text-red-400">Failed to load your labs.</div>
    );
  }
  if (modules && modules.length > 0) {
    return <Navigate to={`/labs/${modules[0].slug}`} replace />;
  }
  // No labs to show. Every signed-in user is entitled to the free first lab, so
  // for a regular user this only happens in a broken/empty catalog; for an admin
  // it means nothing is registered yet. Either way the plain message fits.
  return (
    <div className="flex h-full items-center justify-center p-6">
      <div className="max-w-md text-center">
        <h2 className="text-lg font-semibold text-slate-200">No labs in the catalog</h2>
        <p className="mt-2 text-sm text-slate-500">
          There are no lab modules registered yet. Add one to the backend catalog and
          it&rsquo;ll appear here.
        </p>
      </div>
    </div>
  );
}

export default function App() {
  return (
    <Routes>
      <Route path="login" element={<Login />} />
      <Route index element={<Home />} />
      {/* Public legal stubs, linked from the landing footer. */}
      <Route path="terms" element={<LegalPage title="Terms of Service" />} />
      <Route path="privacy" element={<LegalPage title="Privacy Policy" />} />
      {/* Public - reachable from the landing nav, signed in or not. There's one
          board, so the path is singular; keep the old plural as a redirect so
          existing links/bookmarks don't 404. */}
      <Route path="leaderboard" element={<LeaderboardsPage />} />
      <Route path="leaderboards" element={<Navigate to="/leaderboard" replace />} />
      {/* Public operator profile, linked from the leaderboard and landing deck. */}
      <Route path="u/:username" element={<ProfilePage />} />
      <Route
        element={
          <ProtectedRoute>
            <AppLayout />
          </ProtectedRoute>
        }
      >
        <Route path="labs" element={<LabsIndex />} />
        {/* Post-token "unlock more labs" screen, reached from the sidebar's locked
            Encrypted entry once a user holds at least one lab. Kept a distinct
            route (not /labs, which auto-redirects to the first entitled lab). */}
        <Route path="labs/more" element={<MoreLabsNotice />} />
        {/* Any signed-in user - this is their own account, not an admin surface. */}
        <Route path="settings" element={<SettingsPage />} />
        <Route path="labs/:slug" element={<LabPage />} />
        <Route
          path="admin"
          element={
            <RequireAdmin>
              <AdminPage />
            </RequireAdmin>
          }
        />
        <Route
          path="admin/users"
          element={
            <RequireAdmin>
              <UsersPage />
            </RequireAdmin>
          }
        />
        <Route
          path="admin/labs"
          element={
            <RequireAdmin>
              <LabAdminPage />
            </RequireAdmin>
          }
        />
        <Route
          path="admin/drivers"
          element={
            <RequireAdmin>
              <DriverAdminPage />
            </RequireAdmin>
          }
        />
      </Route>
      {/* The terminal popped into its own window (terminal pane's pop-out
          button). Auth-gated like any lab surface, but deliberately OUTSIDE
          AppLayout: no sidebar or nav chrome, just a full-window shell that
          rebinds to the lab's already-running session. */}
      <Route
        path="labs/:slug/terminal"
        element={
          <ProtectedRoute>
            <PoppedPane pane="terminal" />
          </ProtectedRoute>
        }
      />
      {/* The eBPF sensor feed popped into its own window, for the research /
          eBPF labs - the pop-out mirrors whichever pane was active. */}
      <Route
        path="labs/:slug/defender"
        element={
          <ProtectedRoute>
            <PoppedPane pane="defender" />
          </ProtectedRoute>
        }
      />
      {/* Public catch-all. Kept outside the auth gate so a mistyped URL shows
          the 404 rather than bouncing through /login. */}
      <Route path="*" element={<NotFound />} />
    </Routes>
  );
}
