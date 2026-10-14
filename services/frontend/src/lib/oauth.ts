// Shared OAuth kickoff, used by both the /login card and the landing page's
// "Take the challenge" CTA. This was inline in Login.tsx; it's extracted so the
// marketing landing starts the *exact* same flow - same nonce + state envelope
// the /login callback validates (see Login.tsx's code-exchange effect). Keeping
// one copy means the two entry points can never drift on the state contract.

export type OAuthProvider = "google" | "github";

// Whether a provider is wired in this build. The client IDs are injected at
// build time (VITE_*); locally they may be unset, in which case the button is
// shown disabled rather than bouncing the user to a broken consent screen.
export function oauthConfigured(provider: OAuthProvider): boolean {
  return Boolean(
    provider === "google"
      ? import.meta.env.VITE_GOOGLE_CLIENT_ID
      : import.meta.env.VITE_GITHUB_CLIENT_ID,
  );
}

// Kicks off the redirect to the provider's consent screen. redirectTo is an
// on-site path to land on after the callback completes (null -> the app decides,
// which for a brand-new account is the awaiting-access home). It rides inside
// the signed state param, and the /login callback only honours a same-site path
// (see Login.tsx's safeRedirect), so it can't be used as an open redirect.
export function beginOAuth(provider: OAuthProvider, redirectTo: string | null = null) {
  const clientId =
    (provider === "google"
      ? import.meta.env.VITE_GOOGLE_CLIENT_ID
      : import.meta.env.VITE_GITHUB_CLIENT_ID) ?? "";

  const nonce = crypto.randomUUID();
  sessionStorage.setItem("oauth_state_nonce", nonce);
  const state = btoa(JSON.stringify({ nonce, provider, redirect: redirectTo }));
  const redirectUri = `${window.location.origin}/login`;

  const authUrl =
    provider === "google"
      ? new URL("https://accounts.google.com/o/oauth2/v2/auth")
      : new URL("https://github.com/login/oauth/authorize");

  authUrl.searchParams.set("client_id", clientId);
  authUrl.searchParams.set("redirect_uri", redirectUri);
  authUrl.searchParams.set("state", state);
  if (provider === "google") {
    authUrl.searchParams.set("response_type", "code");
    authUrl.searchParams.set("scope", "openid email profile");
  } else {
    authUrl.searchParams.set("scope", "user:email");
  }

  window.location.href = authUrl.toString();
}
