import TopologyMap from "../components/TopologyMap";
import CapacitySummary from "../components/CapacitySummary";
import { Page, PageSection } from "../layout/Page";

// The admin dashboard is the live cluster topology map. The per-namespace pod
// health table and the Grafana/ArgoCD/Vault/Swagger launcher rings that used
// to live here as separate sections are now folded into the map itself: every
// pod is a status-coloured hex, and any namespace/pod that exposes a dashboard
// carries an inline ↗ link, with ArgoCD-drift / cert-expiry surfaced as chips
// above it (see TopologyMap / topologyRenderer.ts). Kept as one view rather
// than the old three so there's a single source of truth, not the same data
// three times.
export default function AdminPage() {
  return (
    <Page title="Admin">
      <PageSection title="Capacity & health">
        <CapacitySummary />
      </PageSection>
      <PageSection title="Topology">
        <TopologyMap />
      </PageSection>
    </Page>
  );
}
