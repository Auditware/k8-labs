import { createContext, useCallback, useContext, useEffect, useMemo, useState, ReactNode } from "react";
import { getMe, logout as apiLogout, type User } from "../lib/api";

type AuthContextType = {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  // True once the initial auth probe has failed to reach the backend across all
  // its retries (getMe threw every time, as opposed to cleanly returning a 401).
  // The top bar surfaces this as a "server issues" report affordance instead of
  // a "Sign in" button, since the user can't sign in when the server is down.
  serverDown: boolean;
  // Cache-buster for the avatar image URL, shared by every <Avatar>. The avatar
  // endpoint sends a 5-minute max-age, so a re-upload to the same /users/{id}/
  // avatar URL would otherwise keep serving the old bytes until it expires;
  // refreshUser bumps this so the URL changes and the browser refetches.
  avatarVersion: number;
  login: (user: User) => void;
  logout: () => Promise<void>;
  // Re-reads /auth/me into the context. Call after anything that changes the
  // stored user (avatar upload/remove) so hasAvatar updates in the nav too, not
  // just wherever the mutation happened to live.
  refreshUser: () => Promise<void>;
};

const AuthContext = createContext<AuthContextType | undefined>(undefined);

// Delays waited AFTER a failed probe before retrying; the count of attempts is
// this length + 1. Kept short and few - the goal is to ride out a redeploy blip
// or a dropped packet, not to hammer a genuinely dead server.
const PROBE_RETRY_DELAYS = [1000, 2000];

const delay = (ms: number) => new Promise((r) => setTimeout(r, ms));

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [serverDown, setServerDown] = useState(false);
  const [avatarVersion, setAvatarVersion] = useState(0);

  useEffect(() => {
    let cancelled = false;

    (async () => {
      // getMe returns a user or null (401) when the server is reachable, and
      // throws when it isn't. Retry only the throwing case; a null is a real
      // answer ("logged out"), not a failure to get one.
      for (let attempt = 0; ; attempt++) {
        try {
          const u = await getMe();
          if (cancelled) return;
          setUser(u);
          setServerDown(false);
          break;
        } catch {
          if (attempt < PROBE_RETRY_DELAYS.length) {
            await delay(PROBE_RETRY_DELAYS[attempt]);
            if (cancelled) return;
            continue;
          }
          if (cancelled) return;
          setUser(null);
          setServerDown(true);
          break;
        }
      }
      if (!cancelled) setIsLoading(false);
    })();

    return () => {
      cancelled = true;
    };
  }, []);

  // Single-shot, best-effort recovery probe (no retry loop): if the server
  // answers at all - even a 401 - it's reachable, so clear the banner and adopt
  // whatever session it reports. A throw means it's still down; leave state as-is
  // and wait for the next trigger.
  const reprobe = useCallback(async () => {
    try {
      const u = await getMe();
      setUser(u);
      setServerDown(false);
    } catch {
      // still unreachable - keep the banner and try again on the next trigger
    }
  }, []);

  // Recovery from a stale "server issues" state. The initial probe above runs
  // exactly once, so a transient backend blip during first load would otherwise
  // strand the user on the banner for the whole session even after the server
  // came back. While serverDown, re-probe whenever the tab regains focus or
  // becomes visible (the natural "is it back yet?" moments) and on a slow
  // fallback interval for a tab that just sits open. The listeners/interval only
  // exist while down, so a healthy session carries no extra work.
  useEffect(() => {
    if (!serverDown) return;
    const onWake = () => {
      if (document.visibilityState === "visible") void reprobe();
    };
    window.addEventListener("focus", onWake);
    document.addEventListener("visibilitychange", onWake);
    const id = window.setInterval(() => void reprobe(), 15000);
    return () => {
      window.removeEventListener("focus", onWake);
      document.removeEventListener("visibilitychange", onWake);
      window.clearInterval(id);
    };
  }, [serverDown, reprobe]);

  const login = useCallback((u: User) => {
    setUser(u);
    // A successful login is proof the server is back; clear any stale banner.
    setServerDown(false);
  }, []);

  const logout = useCallback(async () => {
    await apiLogout();
    setUser(null);
  }, []);

  const refreshUser = useCallback(async () => {
    // Best-effort: this runs after an action that already succeeded (e.g. an
    // avatar upload), so a transient probe failure shouldn't blow up the caller
    // or eject a still-valid session - keep the current user and move on. A
    // clean 401 (null) still logs out, correctly.
    try {
      setUser(await getMe());
      setServerDown(false);
    } catch {
      // leave user as-is
    }
    setAvatarVersion((v) => v + 1);
  }, []);

  const value = useMemo(
    () => ({
      user,
      isAuthenticated: !!user,
      isLoading,
      serverDown,
      avatarVersion,
      login,
      logout,
      refreshUser,
    }),
    [user, isLoading, serverDown, avatarVersion, login, logout, refreshUser],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within an AuthProvider");
  return ctx;
}
