import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { PostHogProvider } from "@posthog/react";
import { AuthProvider } from "./context/AuthContext";
import App from "./App";
import "./index.css";

const queryClient = new QueryClient();

// Product analytics, PRODUCTION ONLY. The token is a VITE_ build-time var baked
// into the bundle, and CI only supplies it for the prod image (see ci.yml +
// Dockerfile) - so a local `bun dev`/`bun build` has no token and sends
// nothing. The import.meta.env.PROD guard is belt-and-suspenders: even a prod
// build that somehow carried the token wouldn't fire in a dev bundle. phc_ is a
// publishable client key (write-only ingest), so baking it in is by design.
const posthogKey = import.meta.env.VITE_POSTHOG_PROJECT_TOKEN as string | undefined;
const posthogEnabled = import.meta.env.PROD && !!posthogKey;

const app = (
  <QueryClientProvider client={queryClient}>
    <BrowserRouter>
      <AuthProvider>
        <App />
      </AuthProvider>
    </BrowserRouter>
  </QueryClientProvider>
);

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    {posthogEnabled ? (
      <PostHogProvider
        apiKey={posthogKey!}
        options={{
          api_host: (import.meta.env.VITE_POSTHOG_HOST as string) || "https://us.i.posthog.com",
          defaults: "2026-05-30",
        }}
      >
        {app}
      </PostHogProvider>
    ) : (
      app
    )}
  </React.StrictMode>,
);
