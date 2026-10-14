import { useEffect, useRef, useState, type FormEvent } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { googleCallback, githubCallback, devLogin } from "../lib/api";
import { beginOAuth, oauthConfigured } from "../lib/oauth";
import { GoogleIcon, GitHubIcon } from "../components/BrandIcons";
import { Banner } from "../components/ui/Banner";
import { HeroEscapeBackground } from "../components/HeroEscapeBackground";
import { PublicNav } from "../layout/PublicNav";
import { useSeo } from "../lib/seo";

// Only ever navigate to a path on this site - a redirect target from a URL
// query param is attacker-controllable, this is what stops it being used
// to bounce a user off-site after login.
function safeRedirect(to: string | null): string | null {
  if (!to) return null;
  if (to.startsWith("/") && !to.startsWith("//")) return to;
  return null;
}

export default function Login() {
  // Utility route - keep it out of the index.
  useSeo({ title: "Sign in - K8 Labs", path: "/login", noindex: true });
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { login, isAuthenticated, isLoading } = useAuth();
  const [error, setError] = useState("");
  const [pending, setPending] = useState<"google" | "github" | null>(null);
  const handledCode = useRef(false);

  // Dev-only credential login (see below). Prefilled with the defaults the
  // backend accepts so it's a one-click sign-in on localhost.
  const [devUser, setDevUser] = useState("admin");
  const [devPass, setDevPass] = useState("labs");
  const [devBusy, setDevBusy] = useState(false);

  const redirectTo = safeRedirect(searchParams.get("redirect"));
  const code = searchParams.get("code");

  async function submitDevLogin(e: FormEvent) {
    e.preventDefault();
    setError("");
    setDevBusy(true);
    try {
      const user = await devLogin(devUser.trim(), devPass);
      login(user);
      navigate(redirectTo || "/", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setDevBusy(false);
    }
  }

  useEffect(() => {
    if (!isLoading && isAuthenticated) {
      navigate(redirectTo || "/", { replace: true });
    }
  }, [isAuthenticated, isLoading, navigate, redirectTo]);

  useEffect(() => {
    if (!code || handledCode.current) return;
    handledCode.current = true;

    const state = searchParams.get("state");
    let provider: "google" | "github" = "google";
    let targetRedirect = redirectTo;

    if (state) {
      try {
        const stateData = JSON.parse(atob(state));
        const storedNonce = sessionStorage.getItem("oauth_state_nonce");
        sessionStorage.removeItem("oauth_state_nonce");
        if (!storedNonce || stateData.nonce !== storedNonce) {
          setError("Invalid OAuth state - please try signing in again.");
          return;
        }
        if (stateData.provider) provider = stateData.provider;
        if (stateData.redirect) targetRedirect = safeRedirect(stateData.redirect);
      } catch {
        setError("Invalid OAuth state - please try signing in again.");
        return;
      }
    }

    setPending(provider);
    const redirectUri = `${window.location.origin}/login`;
    const exchange = provider === "github" ? githubCallback : googleCallback;

    exchange(code, redirectUri)
      .then((user) => {
        login(user);
        navigate(targetRedirect || "/", { replace: true });
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Sign-in failed. Please try again.");
        setPending(null);
      });
  }, [code]);

  return (
    <div className="relative min-h-screen overflow-x-hidden bg-slate-950 text-slate-100">
      {/* Same ambient escape-grid and top bar as the landing and leaderboard, so
          signing in feels like part of the same surface, not a bare form. */}
      <HeroEscapeBackground className="pointer-events-none fixed inset-0 h-full w-full" />
      <PublicNav />
      <main className="relative z-10 flex min-h-screen items-center justify-center px-4 py-24">
        <div className="w-full max-w-sm rounded-lg border border-slate-800 bg-slate-900 p-8">
        <div className="mb-6 flex items-center gap-2">
          <img src="/logo.svg" alt="" className="h-8 w-auto" />
          <h1 className="text-lg font-semibold text-slate-100">K8 Labs</h1>
        </div>
        <p className="mb-6 text-sm text-slate-400">Sign in to start a lab session.</p>

        {error && (
          <Banner tone="error" className="mb-4">
            {error}
          </Banner>
        )}

        <div className="space-y-3">
          <button
            onClick={() => beginOAuth("google", redirectTo)}
            disabled={pending !== null || !oauthConfigured("google")}
            className="flex h-12 w-full items-center justify-center gap-3 rounded-lg bg-white px-4 text-15
              font-medium text-slate-900 transition-colors hover:bg-slate-100 disabled:opacity-50"
          >
            <GoogleIcon className="h-5 w-5 shrink-0" />
            {pending === "google" ? "Signing in…" : "Continue with Google"}
          </button>
          <button
            onClick={() => beginOAuth("github", redirectTo)}
            disabled={pending !== null || !oauthConfigured("github")}
            className="flex h-12 w-full items-center justify-center gap-3 rounded-lg bg-slate-800 px-4 text-15
              font-medium text-slate-100 ring-1 ring-inset ring-slate-700 transition-colors hover:bg-slate-700 disabled:opacity-50"
          >
            <GitHubIcon className="h-5 w-5 shrink-0" />
            {pending === "github" ? "Signing in…" : "Continue with GitHub"}
          </button>
        </div>

        {(!oauthConfigured("google") || !oauthConfigured("github")) && (
          <p className="mt-4 text-center text-xs text-slate-600">
            {!oauthConfigured("google") && !oauthConfigured("github")
              ? "OAuth not configured"
              : !oauthConfigured("google")
                ? "Google sign-in not configured"
                : "GitHub sign-in not configured"}
          </p>
        )}

        {/* Localhost-only credential login. import.meta.env.DEV is true only
            under `bun run dev`; a production `vite build` compiles this branch
            out entirely, and even if it didn't the backend route is absent
            unless DEV_LOGIN=1. Handy when no OAuth client IDs are configured
            locally and for poking at the app on other devices/viewports. */}
        {import.meta.env.DEV && (
          <form onSubmit={submitDevLogin} className="mt-6 border-t border-slate-800 pt-6">
            <p className="mb-3 text-center font-mono text-11 uppercase tracking-wider text-slate-600">
              dev login · localhost only
            </p>
            <div className="space-y-2">
              <input
                value={devUser}
                onChange={(e) => setDevUser(e.target.value)}
                placeholder="username"
                autoComplete="username"
                aria-label="Dev username"
                className="h-10 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 text-15 text-slate-100 placeholder:text-slate-600 focus:border-accent focus:outline-none"
              />
              <input
                value={devPass}
                onChange={(e) => setDevPass(e.target.value)}
                placeholder="password"
                type="password"
                autoComplete="current-password"
                aria-label="Dev password"
                className="h-10 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 text-15 text-slate-100 placeholder:text-slate-600 focus:border-accent focus:outline-none"
              />
              <button
                type="submit"
                disabled={devBusy}
                className="h-10 w-full rounded-lg bg-accent/15 text-15 font-medium text-accent ring-1 ring-inset ring-accent/30 transition-colors hover:bg-accent/20 disabled:opacity-50"
              >
                {devBusy ? "Signing in…" : "Sign in (dev)"}
              </button>
            </div>
          </form>
        )}
        </div>
      </main>
    </div>
  );
}
