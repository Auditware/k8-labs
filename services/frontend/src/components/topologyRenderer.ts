// Imperative renderer for the admin topology map. Kept out of React on
// purpose: it builds a few hundred absolutely-positioned hex nodes + an SVG
// edge layer and drives its own pan/zoom camera, none of which benefits from
// React's diffing. TopologyMap.tsx fetches the data (react-query) and mounts
// this into a container ref. Design mirrors the backend topology model
// (internal/topology) and the chat mock: node -> namespace (hive) -> pod (mini-hex) -> containers,
// host substrate as layer bands, port-labelled edges from the exact source pod
// to the destination group.
//
// Brand marks: Grafana/Argo/Vault/Swagger are the same CC0 simple-icons the
// app ships in BrandIcons.tsx; the rest are simple hand-drawn shapes. Self-
// contained so the renderer has no React/icon-library dependency.
import type { AdminTopology, TopoPod, NsResources } from "../lib/api";

// A logo is either a hosted brand asset (img: a /logos/*.svg|png served from
// public/, rendered on a light tile so multicolour marks read) or a hand-drawn
// inline shape (path/svg). img wins when present. See public/logos/.
type Logo = { bg: string; fg?: string; path?: string; svg?: string; img?: string };
const LOGO: Record<string, Logo> = {
  cloudflare: { bg: "#F38020", img: "/logos/cloudflare.svg", svg: '<path fill="#fff" d="M15.8 15.5H8.2a2.7 2.7 0 0 1 .4-5.35A4.2 4.2 0 0 1 16 11a2.3 2.3 0 0 1-.2 4.5z"/>' },
  k8s: { bg: "#326CE5", img: "/logos/kubernetes.svg", svg: '<g fill="none" stroke="#fff" stroke-width="1.15"><circle cx="12" cy="12" r="4.3"/><path d="M12 7.7V5.7M12 16.3v2M16.3 12h2M3.7 12h2M15 9l1.5-1.5M7.5 16.5 9 15M15 15l1.5 1.5M7.5 7.5 9 9"/></g><circle cx="12" cy="12" r="1.5" fill="#fff"/>' },
  cilium: { bg: "#F8C517", svg: '<ellipse cx="8.6" cy="9.6" rx="3" ry="1.9" fill="#fff" opacity=".9"/><ellipse cx="15.4" cy="9.6" rx="3" ry="1.9" fill="#fff" opacity=".9"/><ellipse cx="12" cy="13.2" rx="3.1" ry="4.3" fill="#111"/>' },
  postgres: { bg: "#336791", img: "/logos/postgres.svg", svg: '<ellipse cx="12" cy="12" rx="6.2" ry="6.6" fill="none" stroke="#fff" stroke-width="1.2"/><path fill="none" stroke="#fff" stroke-width="1.3" stroke-linecap="round" d="M9.2 11c0-1.6 1.3-2.7 2.9-2.7s2.9 1.1 2.9 2.7-1.3 2.9-2.9 2.9v2.3"/>' },
  k8labs: { bg: "#2e1065", svg: '<ellipse cx="12" cy="9.6" rx="6" ry="5.3" fill="#7c3aed"/><path d="M6.6 12c-1 1.8-1.3 3.3-.9 4.9M12 13.6v3.9M17.4 12c1 1.8 1.3 3.3.9 4.9" stroke="#5b21b6" stroke-width="1.5" fill="none" stroke-linecap="round"/><circle cx="9.9" cy="9.4" r="1.4" fill="#ff3b3b"/><circle cx="14.1" cy="9.4" r="1.4" fill="#ff3b3b"/>' },
  traefik: { bg: "#24A1C1", img: "/logos/traefik.svg", svg: '<path fill="none" stroke="#fff" stroke-width="1.6" stroke-linecap="round" d="M5 8h14M7 12h10M9 16h6"/>' },
  certmanager: { bg: "#2E8B57", img: "/logos/certmanager.svg", svg: '<path fill="none" stroke="#fff" stroke-width="1.6" d="M12 4l6 2v5c0 4-3 6-6 7-3-1-6-3-6-7V6z"/><path fill="none" stroke="#fff" stroke-width="1.6" stroke-linecap="round" d="M9.5 12l1.8 1.8L15 10"/>' },
  loki: { bg: "#F46800", img: "/logos/loki.svg", svg: '<rect x="6" y="6" width="12" height="3" rx="1" fill="#fff"/><rect x="6" y="10.5" width="12" height="3" rx="1" fill="#fff"/><rect x="6" y="15" width="12" height="3" rx="1" fill="#fff"/>' },
  alloy: { bg: "#F46800", img: "/logos/alloy.svg", svg: '<circle cx="8" cy="16" r="2" fill="#fff"/><path fill="none" stroke="#fff" stroke-width="1.6" d="M7 11a6 6 0 0 1 6 6M7 7a10 10 0 0 1 10 10"/>' },
  tetragon: { bg: "#6C4BB6", img: "/logos/tetragon.png", svg: '<circle cx="12" cy="12" r="3" fill="none" stroke="#fff" stroke-width="1.6"/><path fill="none" stroke="#fff" stroke-width="1.6" d="M4 12s3-5 8-5 8 5 8 5-3 5-8 5-8-5-8-5z"/>' },
  session: { bg: "#8B7BE8", svg: '<path fill="none" stroke="#fff" stroke-width="1.5" d="M7 8l-3 4 3 4M17 8l3 4-3 4M13 6l-2 12"/>' },
  k3s: { bg: "#FFC61C", fg: "#243b53", path: "M12 2l9 5v10l-9 5-9-5V7z M12 7l4.5 2.5v5L12 17l-4.5-2.5v-5z" },
  pod: { bg: "#326CE5", img: "/logos/pod.svg", svg: '<circle cx="12" cy="12" r="7" fill="none" stroke="#fff" stroke-width="1.4"/>' },
  plain: { bg: "#2b3640", svg: '<rect x="6" y="6" width="12" height="12" rx="2" fill="none" stroke="#8aa0ae" stroke-width="1.4"/>' },
  // Human actors at the top of the map (see the access-lane block). admin wears
  // the same violet as the privileged/labs surfaces; user is neutral.
  user: { bg: "#2b3640", svg: '<circle cx="12" cy="8.2" r="3.4" fill="#B7C4CE"/><path d="M5.4 19c0-3.7 3-5.7 6.6-5.7s6.6 2 6.6 5.7z" fill="#B7C4CE"/>' },
  admin: { bg: "#6C5CE0", svg: '<circle cx="12" cy="7.6" r="3.2" fill="#fff"/><path d="M5.6 18.6c0-3.5 2.9-5.4 6.4-5.4s6.4 1.9 6.4 5.4z" fill="#fff"/><path d="M12 2.4l2.4.9v1.8c0 1.6-1 2.6-2.4 3.1-1.4-.5-2.4-1.5-2.4-3.1V3.3z" fill="#3b2f8f" stroke="#fff" stroke-width=".7"/>' },
  grafana: { bg: "#F46800", fg: "#fff", path: "M23.02 10.59a8.578 8.578 0 0 0-.862-3.034 8.911 8.911 0 0 0-1.789-2.445c.337-1.342-.413-2.505-.413-2.505-1.292-.08-2.113.4-2.416.62-.052-.02-.102-.044-.154-.064a7.35 7.35 0 0 0-.677-.247 8.9 8.9 0 0 0-.711-.197 9.867 9.867 0 0 0-.875-.161C14.557.753 12.94 0 12.94 0c-1.804 1.145-2.147 2.744-2.147 2.744l-.018.093c-.098.029-.2.057-.298.088-.138.042-.275.094-.413.143-.138.055-.275.107-.41.166a8.869 8.869 0 0 0-1.557.87l-.063-.029c-2.497-.955-4.716.195-4.716.195-.203 2.658.996 4.33 1.235 4.636a11.608 11.608 0 0 0-.607 2.635C1.636 12.677.953 15.014.953 15.014c1.926 2.214 4.171 2.351 4.171 2.351.003-.002.006-.002.006-.005.285.509.615.994.986 1.446.156.19.32.371.488.548-.704 2.009.099 3.68.099 3.68 2.144.08 3.553-.937 3.849-1.173a9.784 9.784 0 0 0 3.164.501h.08l.055-.003.107-.002.103-.005.003.002c1.01 1.44 2.788 1.646 2.788 1.646 1.264-1.332 1.337-2.653 1.337-2.94v-.058c0-.02-.003-.039-.003-.06.265-.187.52-.387.758-.6a7.875 7.875 0 0 0 1.415-1.7c1.43.083 2.437-.885 2.437-.885-.236-1.49-1.085-2.216-1.264-2.354l-.018-.013-.016-.013a.217.217 0 0 1-.031-.02c.008-.092.016-.18.02-.27.011-.162.016-.323.016-.48v-.253l-.005-.098-.008-.135a1.891 1.891 0 0 0-.01-.13c-.003-.042-.008-.083-.013-.125l-.016-.124-.018-.122a6.215 6.215 0 0 0-2.032-3.73 6.015 6.015 0 0 0-3.222-1.46 6.292 6.292 0 0 0-.85-.048l-.107.002h-.063l-.044.003-.104.008a4.777 4.777 0 0 0-3.335 1.695c-.332.4-.592.84-.768 1.297a4.594 4.594 0 0 0-.312 1.817l.003.091c.005.055.007.11.013.164a3.615 3.615 0 0 0 .698 1.82 3.53 3.53 0 0 0 1.827 1.282c.33.098.66.14.971.137.039 0 .078 0 .114-.002l.063-.003c.02 0 .041-.003.062-.003.034-.002.065-.007.099-.01.007 0 .018-.003.028-.003l.031-.005.06-.008a1.18 1.18 0 0 0 .112-.02c.036-.008.072-.013.109-.024a2.634 2.634 0 0 0 .914-.415c.028-.02.056-.041.085-.065a.248.248 0 0 0 .039-.35.244.244 0 0 0-.309-.06l-.078.042c-.09.044-.184.083-.283.116a2.476 2.476 0 0 1-.475.096c-.028.003-.054.006-.083.006l-.083.002c-.026 0-.054 0-.08-.002l-.102-.006h-.012l-.024.006c-.016-.003-.031-.003-.044-.006-.031-.002-.06-.007-.091-.01a2.59 2.59 0 0 1-.724-.213 2.557 2.557 0 0 1-.667-.438 2.52 2.52 0 0 1-.805-1.475 2.306 2.306 0 0 1-.029-.444l.006-.122v-.023l.002-.031c.003-.021.003-.04.005-.06a3.163 3.163 0 0 1 1.352-2.29 3.12 3.12 0 0 1 .937-.43 2.946 2.946 0 0 1 .776-.101h.06l.07.002.045.003h.026l.07.005a4.041 4.041 0 0 1 1.635.49 3.94 3.94 0 0 1 1.602 1.662 3.77 3.77 0 0 1 .397 1.414l.005.076.003.075c.002.026.002.05.002.075 0 .024.003.052 0 .07v.065l-.002.073-.008.174a6.195 6.195 0 0 1-.08.639 5.1 5.1 0 0 1-.267.927 5.31 5.31 0 0 1-.624 1.13 5.052 5.052 0 0 1-3.237 2.014 4.82 4.82 0 0 1-.649.066l-.039.003h-.287a6.607 6.607 0 0 1-1.716-.265 6.776 6.776 0 0 1-3.4-2.274 6.75 6.75 0 0 1-.746-1.15 6.616 6.616 0 0 1-.714-2.596l-.005-.083-.002-.02v-.056l-.003-.073v-.096l-.003-.104v-.07l.003-.163c.008-.22.026-.45.054-.678a8.707 8.707 0 0 1 .28-1.355c.128-.444.286-.872.473-1.277a7.04 7.04 0 0 1 1.456-2.1 5.925 5.925 0 0 1 .953-.763c.169-.111.343-.213.524-.306.089-.05.182-.091.273-.135.047-.02.093-.042.138-.062a7.177 7.177 0 0 1 .714-.267l.145-.045c.049-.015.098-.026.148-.041.098-.029.197-.052.296-.076.049-.013.1-.02.15-.033l.15-.032.151-.028.076-.013.075-.01.153-.024c.057-.01.114-.013.171-.023l.169-.021c.036-.003.073-.008.106-.01l.073-.008.036-.003.042-.002c.057-.003.114-.008.171-.01l.086-.006h.023l.037-.003.145-.007a7.999 7.999 0 0 1 1.708.125 7.917 7.917 0 0 1 2.048.68 8.253 8.253 0 0 1 1.672 1.09l.09.077.089.078c.06.052.114.107.171.159.057.052.112.106.166.16.052.055.107.107.159.164a8.671 8.671 0 0 1 1.41 1.978c.012.026.028.052.04.078l.04.078.075.156c.023.051.05.1.07.153l.065.15a8.848 8.848 0 0 1 .45 1.34.19.19 0 0 0 .201.142.186.186 0 0 0 .172-.184c.01-.246.002-.532-.024-.856z" },
  argocd: { bg: "#EF7B4D", fg: "#fff", path: "M12.581 0c.436.037.871.1 1.299.186 1.679.383 3.121 1.213 4.382 2.365 1.161 1.06 1.917 2.372 2.335 3.881.089.321.216.56.586.624.205.035.238.245.239.43.003.646.002 1.294.002 1.94l-.002 1.21c-.001.356-.116.479-.466.474-.211-.003-.293.119-.344.291-.146.489-.33.966-.552 1.426-.818 1.682-2.084 2.938-3.688 3.87-.077.045-.155.088-.233.131-.252.137-.258.146-.155.415.114.299.358.529.664.625.269.096.553.134.827.21a.672.672 0 0 1 .236.094c-.066.082-.156.067-.231.082-.36.073-.713.184-1.086.17a1.275 1.275 0 0 1-.438-.064c-.114-.045-.152-.006-.176.109a5.354 5.354 0 0 0-.084.92c-.015.617-.071 1.23-.112 1.844-.042.598-.018.651.558.842.281.094.563.187.842.286.069.024.15.038.192.117-.04.057-.098.035-.146.035-.493.003-.985.005-1.478.001-.524-.005-.806-.282-.845-.803-.055-.762-.12-1.524-.182-2.286a.947.947 0 0 0-.026-.12c-.079.455-.065.879-.084 1.298-.023.528-.008 1.057-.007 1.584 0 .27.086.388.335.483.359.135.711.295 1.114.262.141-.012.276.062.402.129.032.017.073.033.069.073-.004.043-.049.047-.084.045-.657-.019-1.317.065-1.972-.028-.323-.046-.533-.236-.631-.552-.094-.303-.114-.617-.137-.93-.046-.626-.078-1.253-.116-1.88a.222.222 0 0 0-.061-.171.282.282 0 0 0-.031.193c-.002.956-.002 1.911-.001 2.866 0 .388.123.575.494.708.481.172.976.298 1.47.423.11.028.225.047.242.192h-1.852c-.051-.01-.103-.022-.155-.03-.701-.1-1.001-.372-1.143-1.042l-.067-.331-.226-1.103c-.069.12-.118.25-.144.386-.083.399-.151.802-.243 1.2-.113.493-.444.763-.932.857l-.33.063H8.558c.057-.171.216-.185.355-.221.476-.127.96-.223 1.417-.409a.603.603 0 0 0 .397-.521c.058-.435.002-.865-.013-1.296a1.528 1.528 0 0 0-.078-.315.405.405 0 0 0-.071.207c-.026.296-.049.591-.075.886-.038.432-.273.716-.679.81a1.702 1.702 0 0 1-.37.045c-.557.003-1.115-.001-1.673-.005-.048 0-.109.019-.148-.065.178-.103.377-.168.582-.187a5.67 5.67 0 0 0 .939-.193c.42-.114.522-.249.512-.687-.023-.931-.091-1.86-.069-2.791.004-.184.001-.368.001-.551a2.387 2.387 0 0 0-.05.385 40.299 40.299 0 0 1-.186 2.623c-.052.513-.296.748-.804.805-.446.051-.889.002-1.332-.02-.108-.006-.234.012-.339-.064.043-.066.106-.07.16-.087.362-.115.725-.224 1.086-.344.246-.081.35-.235.355-.492a2.241 2.241 0 0 0-.003-.232 45.315 45.315 0 0 1-.105-2.149 5.487 5.487 0 0 0-.035-.478c-.024-.188-.131-.287-.295-.258-.505.092-.99-.006-1.473-.139-.059-.016-.134-.007-.178-.088a.986.986 0 0 1 .285-.09c.255-.052.507-.121.753-.208.312-.112.564-.347.695-.651.089-.203.056-.317-.112-.398-1.418-.683-2.512-1.73-3.391-3.017a8.152 8.152 0 0 1-1.123-2.447c-.067-.246-.156-.3-.383-.26-.306.053-.401.006-.535-.273v-3.49c.144-.303.205-.341.534-.329.235.01.247-.004.309-.242.396-1.508 1.082-2.861 2.171-3.988C6.9 1.42 8.523.631 10.34.203c.456-.108.922-.15 1.387-.203h.854Zm7.974 8.948a7.34 7.34 0 0 0-.048-.938 8.353 8.353 0 0 0-.099-.65c-.598-2.964-2.344-5.02-5.051-6.268-1.553-.715-3.21-.835-4.878-.511-3.248.633-5.396 2.583-6.539 5.652-.436 1.173-.495 2.406-.37 3.65.087.935.339 1.846.745 2.694.585 1.213 1.444 2.207 2.477 3.058.343.286.719.528 1.121.719.235.111.247.105.245-.146.006-.16.003-.32-.009-.48-.125-1.02-.142-2.045-.169-3.069a.392.392 0 0 0-.184-.353c-.385-.268-.713-.592-.921-1.019-.474-.97-.372-2.361.813-3.215.136-.097.217-.19.198-.373a1.724 1.724 0 0 1 .031-.442c.177-1.187.748-2.138 1.722-2.84.68-.492 1.442-.772 2.286-.782.483-.007.953.11 1.414.244 1.609.467 2.846 2.07 2.845 3.697a.64.64 0 0 0 .268.565c.463.371.821.83.943 1.426.22 1.077-.083 1.982-.979 2.634-.266.194-.347.406-.333.698.002.047 0 .095-.002.142l-.062 1.439c-.025.586-.138 1.165-.117 1.754.008.223.006.226.201.128a7.46 7.46 0 0 0 2.393-1.903c1.32-1.577 2.074-3.372 2.059-5.511ZM9.117 12.102c1.489.021 2.443-1.578 1.716-2.879a1.937 1.937 0 0 0-1.699-.991c-1.094-.004-1.954.822-1.958 1.881-.005 1.148.813 1.985 1.941 1.989Zm5.794 0c1.101.002 1.935-.823 1.935-1.917 0-1.091-.846-1.949-1.92-1.947-1.064.003-1.94.866-1.943 1.915-.003 1.105.831 1.948 1.928 1.949Zm-1.472 1.937c-.208.128-.407.277-.63.384-.536.257-1.063.257-1.579-.048-.158-.094-.308-.201-.464-.298-.047-.028-.092-.103-.15-.062-.044.03-.01.1-.001.151.037.179.064.362.082.544.027.565.293.992.742 1.31a.984.984 0 0 0 .791.186c.565-.119 1.025-.614 1.124-1.218.043-.266.005-.544.109-.803a.133.133 0 0 0-.024-.146Z" },
  vault: { bg: "#101010", fg: "#FFEC6E", path: "M0 0l11.955 24L24 0zm13.366 4.827h1.393v1.38h-1.393zm-2.77 5.569H9.22V8.993h1.389zm0-2.087H9.22V6.906h1.389zm0-2.086H9.22V4.819h1.389zm2.087 6.263h-1.377V11.08h1.388zm0-2.09h-1.377V8.993h1.388zm0-2.087h-1.377V6.906h1.388zm0-2.086h-1.377V4.819h1.388zm.683.683h1.393v1.389h-1.393zm0 3.475V8.993h1.389v1.388Z" },
  swagger: { bg: "#2b3540", fg: "#85EA2D", path: "M12 0C5.383 0 0 5.383 0 12s5.383 12 12 12c6.616 0 12-5.383 12-12S18.616 0 12 0zm0 1.144c5.995 0 10.856 4.86 10.856 10.856 0 5.995-4.86 10.856-10.856 10.856-5.996 0-10.856-4.86-10.856-10.856C1.144 6.004 6.004 1.144 12 1.144z" },
};

function disc(key: string): string {
  const L = LOGO[key] || LOGO.plain;
  if (L.img) return `<span class="disc img"><img class="dimg" src="${L.img}" alt="" loading="lazy" draggable="false"></span>`;
  const inner = L.path ? `<path fill="${L.fg || "#fff"}" d="${L.path}"/>` : L.svg || "";
  return `<span class="disc" style="background:${L.bg}"><svg viewBox="0 0 24 24" width="17" height="17">${inner}</svg></span>`;
}

function logoForKey(key: string): string {
  const k = key.toLowerCase();
  if (k.includes("grafana")) return "grafana";
  if (k.includes("loki")) return "loki";
  if (k.includes("alloy")) return "alloy";
  if (k.includes("vault")) return "vault";
  if (k.includes("argo")) return "argocd";
  if (k.includes("postgres")) return "postgres";
  if (k.includes("cilium")) return "cilium";
  if (k.includes("traefik")) return "traefik";
  if (k.includes("cert-manager") || k.includes("certmanager")) return "certmanager";
  if (k.includes("tetragon")) return "tetragon";
  if (k.includes("swagger")) return "swagger";
  if (k.includes("backend") || k.includes("frontend")) return "k8labs";
  if (k.includes("runner") || k.includes("session")) return "session";
  if (k.includes("k3s")) return "k3s";
  if (k.startsWith("kube-") || k.includes("etcd") || k.includes("coredns") || k.includes("apiserver") || k.includes("scheduler") || k.includes("controller") || k.includes("local-path"))
    return "k8s";
  return "plain";
}
const podLogo = (p: TopoPod) => { const k = logoForKey(p.app || p.workload || p.name); return k === "plain" ? "pod" : k; };
function nsLogo(name: string): string {
  const map: Record<string, string> = {
    "kube-system": "k8s", default: "k8labs", vault: "vault", argocd: "argocd",
    observability: "grafana", "cert-manager": "certmanager", "labs-runners": "session", traefik: "traefik",
  };
  return map[name] || "k8s";
}

function podStatus(p: TopoPod): "ok" | "warn" | "crit" | "done" {
  const ph = (p.phase || "").toLowerCase();
  if (ph.includes("crash") || ph.includes("error") || ph.includes("backoff") || ph.includes("failed")) return "crit";
  // Succeeded = a Job/init pod that ran to completion (e.g. k3s's
  // helm-install-traefik). Its containers are Terminated so ready is 0/N by
  // design - a finished state, NOT a warning. Show it neutral, not yellow.
  if (ph === "succeeded" || ph === "completed") return "done";
  if (ph === "running") {
    const [r, t] = p.ready.split("/").map(Number);
    return t && r < t ? "warn" : "ok";
  }
  return "warn";
}
const STC: Record<string, string> = { ok: "var(--tp-ok)", warn: "var(--tp-warn)", crit: "var(--tp-crit)", done: "#5b6b7a" };

// Tiny inline icons for the resource hexes (no icon font in this renderer).
const ICO = {
  lock: '<svg viewBox="0 0 12 12"><rect x="2" y="5" width="8" height="6" rx="1" fill="currentColor"/><path d="M3.5 5V3.6a2.5 2.5 0 0 1 5 0V5" fill="none" stroke="currentColor" stroke-width="1.2"/></svg>',
  shield: '<svg viewBox="0 0 12 12"><path d="M6 1l4 1.4v3.1C10 8 8 9.6 6 10.6 4 9.6 2 8 2 5.5V2.4z" fill="currentColor"/></svg>',
  net: '<svg viewBox="0 0 12 12"><circle cx="3" cy="3" r="1.4" fill="currentColor"/><circle cx="9" cy="6.5" r="1.4" fill="currentColor"/><circle cx="4" cy="9.5" r="1.4" fill="currentColor"/><path d="M3.6 3.6 8.4 6M8.4 6.9 4.6 9" stroke="currentColor" stroke-width=".8" fill="none"/></svg>',
  svc: '<svg viewBox="0 0 12 12"><circle cx="6" cy="6" r="3.4" fill="none" stroke="currentColor" stroke-width="1.3"/><circle cx="6" cy="6" r="1" fill="currentColor"/></svg>',
  sa: '<svg viewBox="0 0 12 12"><circle cx="6" cy="4" r="2" fill="currentColor"/><path d="M2.4 10c0-2 1.7-3.1 3.6-3.1S9.6 8 9.6 10z" fill="currentColor"/></svg>',
  cm: '<svg viewBox="0 0 12 12"><rect x="3" y="2" width="6" height="8" rx="1" fill="none" stroke="currentColor" stroke-width="1.2"/><path d="M4.5 4.5h3M4.5 6h3M4.5 7.5h2" stroke="currentColor" stroke-width=".9"/></svg>',
};
type ResKind = { count: number; color: string; ico: string; label: string };
// One entry per present resource kind - each renders as a mini-hex inside the
// namespace hive (alongside the pods), so resources read as members of the
// namespace just like containers, not a badge bolted onto the header.
function resourceKinds(r?: NsResources): ResKind[] {
  if (!r) return [];
  const out: ResKind[] = [];
  if (r.secrets) out.push({ count: r.secrets, color: "var(--tp-warn)", ico: ICO.lock, label: "secrets" });
  if (r.roles) out.push({ count: r.roles, color: "var(--tp-violet)", ico: ICO.shield, label: "roles" });
  if (r.roleBindings) out.push({ count: r.roleBindings, color: "var(--tp-violet)", ico: ICO.shield, label: "rolebindings" });
  if (r.networkPolicies) out.push({ count: r.networkPolicies, color: "var(--tp-accent)", ico: ICO.net, label: "networkpolicies" });
  if (r.services) out.push({ count: r.services, color: "var(--tp-ink2)", ico: ICO.svc, label: "services" });
  if (r.serviceAccounts) out.push({ count: r.serviceAccounts, color: "var(--tp-ink2)", ico: ICO.sa, label: "serviceaccounts" });
  if (r.configMaps) out.push({ count: r.configMaps, color: "var(--tp-ink2)", ico: ICO.cm, label: "configmaps" });
  return out;
}
// Full inventory for the popover.
function resInventory(r?: NsResources): string {
  if (!r) return "";
  const kv = (label: string, n?: number, hot = false) => (n ? `<span class="kv${hot ? " hot" : ""}"><b>${n}</b> ${label}</span>` : "");
  const inv = [
    kv("secrets", r.secrets, true), kv("roles", r.roles, true), kv("rolebindings", r.roleBindings, true), kv("netpol", r.networkPolicies, true),
    kv("services", r.services), kv("serviceaccounts", r.serviceAccounts), kv("configmaps", r.configMaps),
  ].filter(Boolean).join("");
  if (!inv) return "";
  const names = r.secretNames?.length ? `<div class="secnames">secrets: ${r.secretNames.map((n) => `<b>${n}</b>`).join(", ")}</div>` : "";
  return `<div class="inv">${inv}</div>${names}`;
}

// ---- geometry (flat-top hex disk packing, same as the mock) ----
const S = 22, MINw = 2 * S, MINh = Math.round(Math.sqrt(3) * S), HX = 1.5 * S, HY = Math.sqrt(3) * S, HEADH = 40;
function axialDisk(n: number) {
  const out = [{ q: 0, r: 0 }], dirs = [[1, 0], [1, -1], [0, -1], [-1, 0], [-1, 1], [0, 1]];
  let rad = 1;
  while (out.length < n) {
    let q = -rad, r = rad;
    for (let side = 0; side < 6 && out.length < n; side++)
      for (let step = 0; step < rad && out.length < n; step++) { out.push({ q, r }); q += dirs[side][0]; r += dirs[side][1]; }
    rad++;
  }
  return out.slice(0, n);
}
function podCells(n: number) {
  if (n <= 0) return { w: 90, h: 78, cells: [] as { cx: number; cy: number }[] };
  const ax = axialDisk(n).map((c) => ({ x: HX * c.q, y: HY * (c.r + c.q / 2) }));
  let mnx = 1e9, mny = 1e9, mxx = -1e9, mxy = -1e9;
  ax.forEach((p) => { mnx = Math.min(mnx, p.x); mny = Math.min(mny, p.y); mxx = Math.max(mxx, p.x); mxy = Math.max(mxy, p.y); });
  const pad = 16;
  const h = Math.max(mxy - mny + MINh + pad * 2, 72);
  // A flat-top hexagon is naturally wider than tall (2s vs sqrt(3)s ~= 1.15x),
  // but the pod-disk packing is taller than wide, so the hex renders too tall.
  // Floor the width at that flat-top ratio and centre the disk in the extra
  // width, so hives read as hexagons rather than upright capsules.
  const contentW = mxx - mnx + MINw + pad * 2;
  const w = Math.max(contentW, 80, Math.round(h * 1.15));
  const shiftX = (w - contentW) / 2;
  return { w, h, cells: ax.map((p) => ({ cx: pad + MINw / 2 + (p.x - mnx) + shiftX, cy: pad + MINh / 2 + (p.y - mny) })) };
}

type Hive = { id: string; name: string; logo: string; pods: TopoPod[]; dash?: { label: string; path: string }; resources?: NsResources };
type RNode = { name: string; role: string; ready: boolean; kernel: string; hives: Hive[] };

const STYLE = `
.tp-root{position:relative;height:100%;min-height:520px;overflow:hidden;color:#E4EBF0;font-family:system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;
  --tp-line:#222C34;--tp-line2:#2E3A44;--tp-line3:#3a4650;--tp-ink:#E4EBF0;--tp-ink2:#9AA9B5;--tp-ink3:#66747F;--tp-accent:#2FD6C6;--tp-violet:#8B7BE8;--tp-ok:#49C97E;--tp-warn:#E7B24A;--tp-crit:#EC5C63;--tp-ext:#5AA0EA;--tp-stage:#0d1216;
  background:radial-gradient(circle at 1px 1px,rgba(255,255,255,.03) 1px,transparent 0) 0 0/24px 24px,var(--tp-stage)}
.tp-root .cam{position:absolute;inset:0;cursor:grab}.tp-root .cam.drag{cursor:grabbing}
.tp-root .stage{position:absolute;top:0;left:0;transform-origin:0 0}
/* Edges paint ABOVE the hives (z 3) so a link is never hidden behind a
   hexagon - you can always trace source -> dest. pointer-events:none so the
   overlay never eats a click meant for a hive underneath. */
.tp-root svg.edges{position:absolute;top:0;left:0;overflow:visible;z-index:6;pointer-events:none}
.tp-root svg.edges text{paint-order:stroke;stroke:var(--tp-stage);stroke-width:3.5px;stroke-linejoin:round}
.tp-root .region{position:absolute;z-index:1;border:1px solid var(--tp-line2);border-radius:22px;background:rgba(255,255,255,.014)}
.tp-root .region.control-plane{border-top:2px solid var(--tp-accent)}.tp-root .region.labs{border-top:2px solid var(--tp-violet)}.tp-root .region.nested{border:1.5px solid var(--tp-violet);background:rgba(139,123,232,.05)}
.tp-root .rhead{position:absolute;top:-16px;left:24px;display:flex;align-items:center;gap:10px;background:var(--tp-stage);padding:3px 13px 3px 5px;border-radius:12px}
.tp-root .rhead .nb{width:28px;height:28px;border-radius:8px;display:flex;align-items:center;justify-content:center}.tp-root .rhead .tt{font-size:14px;font-weight:600;line-height:1.1}.tp-root .rhead .mm{font-size:11px;color:var(--tp-ink3)}
.tp-root .hive{position:absolute;z-index:3;cursor:pointer;transition:opacity .15s}.tp-root .hive.dim{opacity:.24}
/* Header (icon + name + ↗) is centred over the hive, since the hex below is
   centred in the hive's width - left-aligning the header left it visibly off to
   one side of its own honeycomb. */
.tp-root .hhead{display:flex;align-items:center;justify-content:center;gap:8px;margin-bottom:6px;padding:0 2px}
.tp-root .disc{width:28px;height:28px;border-radius:8px;display:flex;align-items:center;justify-content:center;flex:none}
.tp-root .disc.img{background:transparent}
.tp-root .disc .dimg{width:92%;height:92%;object-fit:contain;display:block}
.tp-root .hn{font:500 12.5px ui-monospace,Menlo,monospace;color:var(--tp-ink);line-height:1.05;white-space:nowrap}.tp-root .hc{font:400 10px ui-monospace,Menlo,monospace;color:var(--tp-ink3);display:flex;align-items:center;gap:6px}
.tp-root .rb{display:inline-flex;align-items:center;gap:2px;font:500 9px ui-monospace,Menlo,monospace;border-radius:5px;padding:0 4px;line-height:14px;background:#12181d;border:1px solid var(--tp-line2)}
.tp-root .rb svg{width:9px;height:9px}
.tp-root .rb.sec{color:var(--tp-warn);border-color:rgba(231,178,74,.35)}.tp-root .rb.rbac{color:var(--tp-violet);border-color:rgba(139,123,232,.35)}.tp-root .rb.np{color:var(--tp-accent);border-color:rgba(47,214,198,.3)}
.tp-root .pop .inv{margin-top:8px;border-top:1px solid var(--tp-line);padding-top:8px;display:flex;flex-wrap:wrap;gap:5px}
.tp-root .pop .inv .kv{display:inline-flex;align-items:center;gap:4px;font:400 10.5px ui-monospace,Menlo,monospace;color:var(--tp-ink2);background:#0f151a;border:1px solid var(--tp-line);border-radius:6px;padding:2px 7px}
.tp-root .pop .inv .kv b{font-weight:600;color:var(--tp-ink)}.tp-root .pop .inv .kv.hot{color:var(--tp-warn);border-color:rgba(231,178,74,.3)}
.tp-root .pop .secnames{margin-top:6px;font:400 10.5px ui-monospace,Menlo,monospace;color:var(--tp-ink3)}.tp-root .pop .secnames b{color:var(--tp-warn);font-weight:500}
/* Dashboard ↗ sits right after the namespace name, not shoved to the far right
   of the (often much wider) hive header - so it reads as belonging to this
   namespace's icon rather than floating in space above a neighbour. */
.tp-root .hdash{margin-left:6px;align-self:center;color:var(--tp-accent);font-size:12px;flex:none;text-decoration:none}
.tp-root .bighex .bg{position:absolute;inset:0;clip-path:polygon(25% 0,75% 0,100% 50%,75% 100%,25% 100%,0 50%);background:var(--tp-line2)}
.tp-root .bighex .in{position:absolute;inset:1.8px;clip-path:polygon(25% 0,75% 0,100% 50%,75% 100%,25% 100%,0 50%);background:#141b21}
.tp-root .hive.ok .bg{background:var(--tp-ok)}.tp-root .hive.warn .bg{background:var(--tp-warn)}.tp-root .hive.crit .bg{background:var(--tp-crit)}
.tp-root .hive:hover .in{background:#1a222a}
.tp-root .mini{position:absolute;z-index:1;cursor:pointer;transition:opacity .15s}.tp-root .mini.dim{opacity:.16}
.tp-root .mini .mbg{position:absolute;inset:0;clip-path:polygon(25% 0,75% 0,100% 50%,75% 100%,25% 100%,0 50%)}
.tp-root .mini .min{position:absolute;inset:1.6px;clip-path:polygon(25% 0,75% 0,100% 50%,75% 100%,25% 100%,0 50%);display:flex;align-items:center;justify-content:center;background:#141b21}
.tp-root .mini .min .disc{width:23px;height:23px;border-radius:6px}.tp-root .mini .min .disc svg{width:15px;height:15px}
.tp-root .mini.res .min{flex-direction:column;gap:0}.tp-root .mini.res .ric{display:flex}.tp-root .mini.res .ric svg{width:13px;height:13px}.tp-root .mini.res .rc{font:600 9px ui-monospace,Menlo,monospace;line-height:1;margin-top:1px}
.tp-root .mini:hover .min{filter:brightness(1.35)}
.tp-root .layer{position:absolute;z-index:1;display:flex;align-items:center;gap:10px;height:40px;border-radius:9px;border:1px solid var(--tp-line);background:linear-gradient(90deg,rgba(255,255,255,.03),rgba(255,255,255,.008));padding:0 14px;overflow:hidden}
.tp-root .layer .lt{font-size:12px;font-weight:500;color:var(--tp-ink);white-space:nowrap;flex:none}.tp-root .layer .ls{font:400 11px ui-monospace,Menlo,monospace;color:var(--tp-ink3);margin-left:auto;white-space:nowrap;padding-left:16px;min-width:0;overflow:hidden;text-overflow:ellipsis;text-align:right}
.tp-root .layer.net{border-color:var(--tp-accent)}.tp-root .layer.net .lt{color:var(--tp-accent)}.tp-root .layer.wall{border-color:var(--tp-violet)}.tp-root .layer.wall .lt{color:var(--tp-violet)}.tp-root .layer.warn{border-color:rgba(231,178,74,.45)}.tp-root .layer.warn .lt{color:var(--tp-warn)}
.tp-root .subhdr{position:absolute;z-index:1;font-size:10.5px;font-weight:500;line-height:1.5;letter-spacing:.6px;text-transform:uppercase;color:var(--tp-ink3);white-space:nowrap}
.tp-root .ctrls{position:absolute;left:16px;bottom:16px;z-index:20;display:flex;flex-direction:column;gap:5px}
.tp-root .ctrls button{width:34px;height:34px;background:#12181d;border:1px solid var(--tp-line);color:var(--tp-ink2);border-radius:9px;cursor:pointer;font-size:16px;line-height:1}
.tp-root .legend{position:absolute;right:16px;bottom:16px;z-index:20;display:flex;flex-wrap:wrap;gap:13px;font-size:10.5px;color:var(--tp-ink2);background:rgba(11,14,17,.8);border:1px solid var(--tp-line);border-radius:8px;padding:7px 12px}
.tp-root .legend span{display:inline-flex;align-items:center;gap:6px}
.tp-root .legend i{width:18px;height:0;border-top:2px dashed currentColor;display:inline-block}
.tp-root .pop{position:absolute;z-index:30;min-width:224px;max-width:280px;background:#12181d;border:1px solid var(--tp-line2);border-radius:12px;padding:13px 14px;box-shadow:0 12px 34px rgba(0,0,0,.6)}
.tp-root .pop h4{margin:0 0 2px;font-size:13.5px;font-weight:600;display:flex;align-items:center;gap:8px}.tp-root .pop h4 .disc{width:22px;height:22px}
.tp-root .pop h4 .ricbig{width:22px;height:22px;display:flex;align-items:center;justify-content:center;flex:none}.tp-root .pop h4 .ricbig svg{width:18px;height:18px}
.tp-root .pop .ps{font-size:11px;color:var(--tp-ink3);margin-bottom:9px}
.tp-root .pop .dashbtn{display:flex;align-items:center;gap:8px;background:rgba(47,214,198,.1);border:1px solid rgba(47,214,198,.3);color:var(--tp-accent);border-radius:8px;padding:7px 10px;font-size:12px;margin-bottom:10px;text-decoration:none}
.tp-root .pop .row{display:flex;align-items:center;gap:8px;font-size:11.5px;padding:5px 0;border-top:1px solid var(--tp-line)}
.tp-root .pop .row .pd{width:7px;height:7px;border-radius:50%;flex:none}.tp-root .pop .row .pc{margin-left:auto;font:400 10px ui-monospace,Menlo,monospace;color:var(--tp-ink3)}
.tp-root .pop .pn{font-family:ui-monospace,Menlo,monospace;font-size:11px}
.tp-root .pop .clist{margin:6px 0 0;padding-left:15px}.tp-root .pop .clist li{font:400 10.5px ui-monospace,Menlo,monospace;color:var(--tp-ink2);padding:2px 0}
.tp-root .pop .cl{position:absolute;top:9px;right:11px;color:var(--tp-ink3);cursor:pointer}
.tp-root .empty{position:absolute;inset:0;display:flex;align-items:center;justify-content:center;color:var(--tp-ink3);font-size:13px}
/* Human/external actors in the top access lane. Rounded (not hex) so they read
   as "outside the cluster" vs the hexagon namespaces. .ext (Cloudflare) gets a
   dashed orange rim - it sits off-cluster, in front of the origin. */
.tp-root .actor{position:absolute;z-index:4;display:flex;align-items:center;gap:9px;background:#12181d;border:1px solid var(--tp-line2);border-radius:12px;padding:8px 13px 8px 9px;min-width:118px}
.tp-root .actor.ext{border-style:dashed;border-color:rgba(243,128,32,.7)}
.tp-root .actor .disc{width:26px;height:26px;border-radius:7px}
.tp-root .actor .an{font:600 12.5px system-ui,-apple-system,sans-serif;color:var(--tp-ink);line-height:1.1}
.tp-root .actor .as{font:400 10px ui-monospace,Menlo,monospace;color:var(--tp-ink3);margin-top:2px;white-space:nowrap}
`;

function ensureStyle() {
  if (document.getElementById("tp-style")) return;
  const s = document.createElement("style");
  s.id = "tp-style";
  s.textContent = STYLE;
  document.head.appendChild(s);
}

// Group the flat namespaces[] into per-node regions by each pod's .node.
function groupByNode(data: AdminTopology): RNode[] {
  const order = (r: string) => (r === "control-plane" ? 0 : r === "labs" ? 1 : 2);
  const nodes = new Map<string, RNode>();
  for (const n of data.nodes ?? []) nodes.set(n.name, { name: n.name, role: n.role, ready: n.ready, kernel: n.kernel, hives: [] });
  const fallback = [...nodes.values()].sort((a, b) => order(a.role) - order(b.role))[0];
  // node -> namespace -> pods
  const byNode = new Map<string, Map<string, TopoPod[]>>();
  const dashByRef = new Map<string, { label: string; path: string }>();
  for (const d of data.dashboards ?? []) dashByRef.set(d.ref, { label: d.label, path: d.path });
  const resByNs = new Map<string, NsResources>();
  for (const ns of data.namespaces ?? []) if (ns.resources) resByNs.set(ns.name, ns.resources);
  for (const ns of data.namespaces ?? []) {
    for (const p of ns.pods ?? []) {
      const nn = (p.node && nodes.has(p.node) ? p.node : fallback?.name) || "unscheduled";
      if (!byNode.has(nn)) byNode.set(nn, new Map());
      const m = byNode.get(nn)!;
      if (!m.has(ns.name)) m.set(ns.name, []);
      m.get(ns.name)!.push(p);
    }
  }
  for (const [nn, nsMap] of byNode) {
    let node = nodes.get(nn);
    if (!node) { node = { name: nn, role: "worker", ready: true, kernel: "", hives: [] }; nodes.set(nn, node); }
    const names = [...nsMap.keys()].sort((a, b) => (a === "kube-system" ? 1 : b === "kube-system" ? -1 : a.localeCompare(b)));
    for (const name of names) {
      const pods = nsMap.get(name)!;
      // hive-level dashboard: any dashboard whose ref matches a pod in the hive
      let dash: { label: string; path: string } | undefined;
      for (const p of pods) {
        const key = p.app || p.workload;
        for (const [ref, d] of dashByRef) if (key === ref || key.startsWith(ref) || p.name.startsWith(ref)) { dash = d; break; }
        if (dash) break;
      }
      node.hives.push({ id: nn + "/" + name, name, logo: nsLogo(name), pods, dash, resources: resByNs.get(name) });
    }
  }
  return [...nodes.values()].filter((n) => n.hives.length > 0).sort((a, b) => order(a.role) - order(b.role));
}

// opts.scoped renders the STUDENT-facing map. The scoped payload is already
// free of outer-cluster data (internal/topology/scoped.go), but this renderer
// draws chrome of its own that no payload controls - the access lane naming an
// admin with an allowlisted IP, the host-substrate band, the isolation-wall
// panel naming the runtime and the NetworkPolicy architecture, and a legend
// describing host access. All of that is infrastructure disclosure, so scoped
// mode suppresses it. Found by looking at the real thing: the payload was
// clean and the picture still told the student we run runc with no microVM.
export function renderTopology(
  container: HTMLElement,
  data: AdminTopology,
  opts: { scoped?: boolean } = {},
): () => void {
  const scoped = opts.scoped === true;
  ensureStyle();
  container.innerHTML = "";
  container.classList.add("tp-root");
  const rnodes = groupByNode(data);
  if (rnodes.length === 0) {
    container.innerHTML = `<div class="empty">No cluster data - the backend returned an empty topology.</div>`;
    return () => { container.innerHTML = ""; };
  }

  const cam = document.createElement("div"); cam.className = "cam";
  const stage = document.createElement("div"); stage.className = "stage";
  const NS = "http://www.w3.org/2000/svg";
  const edges = document.createElementNS(NS, "svg"); edges.setAttribute("class", "edges"); edges.setAttribute("width", "3000"); edges.setAttribute("height", "2200");
  stage.appendChild(edges); cam.appendChild(stage); container.appendChild(cam);
  edges.innerHTML = `<defs>
    <marker id="tp-a" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto"><path d="M0 0 6 3 0 6Z" fill="var(--tp-ink3)"/></marker>
    <marker id="tp-av" markerWidth="9" markerHeight="9" refX="6" refY="3" orient="auto"><path d="M0 0 6 3 0 6Z" fill="var(--tp-violet)"/></marker>
    <marker id="tp-ae" markerWidth="9" markerHeight="9" refX="6" refY="3" orient="auto"><path d="M0 0 6 3 0 6Z" fill="#ff5b6b"/></marker>
    <marker id="tp-aext" markerWidth="9" markerHeight="9" refX="6" refY="3" orient="auto"><path d="M0 0 6 3 0 6Z" fill="var(--tp-ext)"/></marker>
    <!-- observes: an eye, not an arrowhead. A sensor watching a pod is not
         traffic reaching it, and an arrow would read as a network hop. Left
         unrotated (no orient) so the eye stays upright whatever the approach
         angle - a tilted eye reads as a glyph, not an eye. -->
    <marker id="tp-eye" markerWidth="14" markerHeight="10" refX="7" refY="5"><path d="M1 5Q7 1 13 5Q7 9 1 5Z" fill="none" stroke="var(--tp-accent)" stroke-width="1.2"/><circle cx="7" cy="5" r="1.7" fill="var(--tp-accent)"/></marker></defs>`;

  // Prod runs behind Cloudflare (Cloudflare-only origin firewall) and isolates
  // lab pods in kata-fc Firecracker microVMs; local k3d has neither. The
  // Firecracker RuntimeClass shows up as a substrate layer, so its presence is
  // the same live "is this prod" fact the layer bands already trust - reused
  // here to decide whether to draw the Cloudflare gateway + SSH path.
  const kata = (data.layers || []).some((l) => l.nodeRole === "labs" && /kata/i.test(l.title || ""));

  const pos: Record<string, { cx: number; cy: number }> = {};
  // Per-hive hexagon geometry (centre + half-extents of the honeycomb), so an
  // edge can land its arrow ON the destination hexagon's border rather than
  // somewhere inside it. Half-extents match the bighex box (m.pc.w/h).
  const hiveGeom: Record<string, { cx: number; cy: number; halfW: number; halfH: number }> = {};
  const podIndex: { key: string; name: string; pid: string; hiveId: string; center: { cx: number; cy: number }; hiveCenter: { cx: number; cy: number } }[] = [];
  // Absolute centre of each hive's "secrets" resource hex, so an escape edge
  // can point at the secret it reads rather than at the namespace outline.
  const secretIndex: Record<string, { cx: number; cy: number }> = {};
  const measure = (h: Hive) => { const pc = podCells(h.pods.length + resourceKinds(h.resources).length); return { pc, w: Math.max(pc.w, 158), totalH: HEADH + pc.h }; };

  let popEl: HTMLElement | null = null;
  const closePop = () => { if (popEl) { popEl.remove(); popEl = null; } };
  function popShell(inner: string, anchor: HTMLElement) {
    closePop();
    popEl = document.createElement("div"); popEl.className = "pop"; popEl.innerHTML = `<span class="cl">✕</span>${inner}`;
    const wr = container.getBoundingClientRect(), ar = anchor.getBoundingClientRect();
    container.appendChild(popEl);
    let lx = ar.right - wr.left + 8; if (lx + 280 > wr.width) lx = ar.left - wr.left - 288;
    popEl.style.left = Math.max(8, lx) + "px"; popEl.style.top = Math.max(8, Math.min(ar.top - wr.top, wr.height - 240)) + "px";
    (popEl.querySelector(".cl") as HTMLElement).onclick = closePop;
    popEl.onmouseleave = closePop;
  }
  function podPop(p: TopoPod, anchor: HTMLElement) {
    const st = podStatus(p);
    popShell(`<h4>${disc(podLogo(p))}${p.name}</h4><div class="ps">pod · ${p.containers.length} container${p.containers.length > 1 ? "s" : ""} · ${p.ready} · ${p.phase}</div>
      <ul class="clist">${p.containers.map((c) => `<li>${c}</li>`).join("")}</ul>`, anchor);
    void st;
  }
  function hivePop(h: Hive, anchor: HTMLElement) {
    popShell(`<h4>${disc(h.logo)}${h.name}</h4><div class="ps">namespace · ${h.pods.length} pod${h.pods.length > 1 ? "s" : ""}</div>
      ${h.dash ? `<a class="dashbtn" href="${h.dash.path}" target="_blank" rel="noreferrer">↗ ${h.dash.label}</a>` : ""}
      ${h.pods.map((p) => `<div class="row"><span class="pd" style="background:${STC[podStatus(p)]}"></span><span class="pn">${p.name}</span><span class="pc">${p.ready}</span></div>`).join("")}
      ${resInventory(h.resources)}`, anchor);
  }
  // Clicking a single resource hex focuses just that kind (not the whole hive).
  function resPop(h: Hive, ri: number, anchor: HTMLElement) {
    const rk = resourceKinds(h.resources)[ri];
    if (!rk) return hivePop(h, anchor);
    const secret = rk.label === "secrets";
    const names = secret && h.resources?.secretNames?.length
      ? `<ul class="clist">${h.resources.secretNames.map((n) => `<li>${n}</li>`).join("")}</ul>`
      : `<div class="ps">names not enumerated for this kind</div>`;
    popShell(`<h4><span class="ricbig" style="color:${rk.color}">${rk.ico}</span>${rk.count} ${rk.label}</h4>
      <div class="ps">${h.name} · ${secret ? "names only, values never read" : rk.label}</div>${names}`, anchor);
  }

  function hiveEl(h: Hive, x: number, y: number, m: ReturnType<typeof measure>) {
    const el = document.createElement("div");
    const st = h.pods.some((p) => podStatus(p) === "crit") ? "crit" : h.pods.some((p) => podStatus(p) === "warn") ? "warn" : "ok";
    el.className = "hive " + st; el.style.left = x + "px"; el.style.top = y + "px"; el.style.width = m.w + "px"; el.dataset.id = h.id;
    const hexX = (m.w - m.pc.w) / 2;
    const cell = (c: { cx: number; cy: number }) => `left:${hexX + c.cx - MINw / 2}px;top:${HEADH + c.cy - MINh / 2}px;width:${MINw}px;height:${MINh}px`;
    let minis = "";
    h.pods.forEach((p, i) => {
      const c = m.pc.cells[i]; if (!c) return;
      minis += `<div class="mini" data-pi="${i}" data-pod="${h.id}#${i}" title="${p.name}" style="${cell(c)}">
        <div class="mbg" style="background:${STC[podStatus(p)]}"></div><div class="min">${disc(podLogo(p))}</div></div>`;
    });
    // Resource kinds pack into the same honeycomb after the pods - members of
    // the namespace, styled distinctly (kind icon + count, rim in the kind
    // colour) so they read apart from status-coloured pod hexes.
    const kinds = resourceKinds(h.resources);
    kinds.forEach((rk, j) => {
      const c = m.pc.cells[h.pods.length + j]; if (!c) return;
      if (rk.label === "secrets") secretIndex[h.id] = { cx: x + hexX + c.cx, cy: y + HEADH + c.cy };
      minis += `<div class="mini res" data-ri="${j}" title="${rk.count} ${rk.label}" style="${cell(c)}">
        <div class="mbg" style="background:${rk.color}"></div><div class="min" style="color:${rk.color}"><span class="ric">${rk.ico}</span><span class="rc">${rk.count}</span></div></div>`;
    });
    const podLabel = h.pods.length === 1 ? "1 pod" : `${h.pods.length} pods`;
    el.innerHTML = `<div class="hhead">${disc(h.logo)}<div><div class="hn">${h.name}</div><div class="hc"><span>${podLabel}</span></div></div>${h.dash ? `<a class="hdash" href="${h.dash.path}" target="_blank" rel="noreferrer">↗</a>` : ""}</div>
      <div class="bighex" style="position:absolute;left:${hexX}px;top:${HEADH}px;width:${m.pc.w}px;height:${m.pc.h}px"><div class="bg"></div><div class="in"></div></div>${minis}`;
    el.addEventListener("mouseenter", () => highlight(h.id));
    el.addEventListener("mouseleave", () => highlight(null));
    el.addEventListener("click", (e) => { e.stopPropagation(); hivePop(h, el); });
    el.querySelectorAll(".mini").forEach((mn) => {
      const el2 = mn as HTMLElement;
      mn.addEventListener("click", (e) => {
        e.stopPropagation();
        if (el2.dataset.pi !== undefined) podPop(h.pods[+el2.dataset.pi], el2);
        else if (el2.dataset.ri !== undefined) resPop(h, +el2.dataset.ri, el2); // resource hex -> that kind
        else hivePop(h, el2);
      });
      // Pod-level hover isolates just this pod's links; leaving it falls back to
      // the hive-level focus (the cursor is still inside the hive).
      if (el2.dataset.pod) {
        mn.addEventListener("mouseenter", (e) => { e.stopPropagation(); highlightPod(el2.dataset.pod!); });
        mn.addEventListener("mouseleave", () => highlight(h.id));
      }
    });
    pos[h.id] = { cx: x + m.w / 2, cy: y + HEADH + m.pc.h / 2 };
    hiveGeom[h.id] = { cx: x + m.w / 2, cy: y + HEADH + m.pc.h / 2, halfW: m.pc.w / 2, halfH: m.pc.h / 2 };
    // index each pod for edge resolution
    h.pods.forEach((p, i) => {
      const c = m.pc.cells[i]; if (!c) return;
      podIndex.push({ key: (p.app || p.workload || p.name).toLowerCase(), name: p.name.toLowerCase(), pid: h.id + "#" + i, hiveId: h.id, center: { cx: x + hexX + c.cx, cy: y + HEADH + c.cy }, hiveCenter: pos[h.id] });
    });
    return el;
  }

  function drawNode(node: RNode, ox: number, oy: number) {
    const COLGAP = 62, PADX = 32, TOPPAD = 52, HIVE_GAP = 44, PERCOL = 2;
    const cols: Hive[][] = [];
    node.hives.forEach((h, i) => { const c = Math.floor(i / PERCOL); (cols[c] ||= []).push(h); });
    const measured = cols.map((col) => col.map(measure));
    const colW = cols.map((_, ci) => Math.max(...measured[ci].map((m) => m.w), 158));
    const colX: number[] = []; let cx = ox + PADX; colW.forEach((w, i) => { colX[i] = cx; cx += w + COLGAP; });
    // floor wide enough that the host-substrate bands (e.g. the kata-fc line)
    // fit without their right-aligned sub-text overflowing the region border.
    const innerW = Math.max(cx - COLGAP - (ox + PADX), 520);
    const colBottom: number[] = [];
    cols.forEach((col, ci) => {
      let y = oy + TOPPAD;
      col.forEach((h, hi) => { const m = measured[ci][hi]; stage.appendChild(hiveEl(h, colX[ci] + (colW[ci] - m.w) / 2, y, m)); y += m.totalH + HIVE_GAP; });
      colBottom[ci] = y - HIVE_GAP;
    });
    const subY = Math.max(...colBottom) + 50;
    // The host substrate is the machine under the platform - kernel, runtime,
    // firewall. Never shown to a student.
    if (!scoped) {
      const sub = document.createElement("div"); sub.className = "subhdr"; sub.textContent = "host substrate";
      sub.style.left = ox + PADX + "px"; sub.style.top = subY - 24 + "px"; sub.style.width = innerW + "px"; stage.appendChild(sub);
    }
    let ly = subY;
    for (const L of scoped ? [] : data.layers.filter((l) => l.nodeRole === node.role)) {
      const el = document.createElement("div"); el.className = "layer " + (L.kind || "");
      el.style.left = ox + PADX + "px"; el.style.top = ly + "px"; el.style.width = innerW + "px";
      const subTxt = L.icon === "kernel" && node.kernel ? "host · " + node.kernel : L.sub || "";
      el.innerHTML = `<span class="lt">${L.title}</span><span class="ls">${subTxt}</span>`;
      stage.appendChild(el); ly += 48;
    }
    const regionH = ly - 8 - oy + 44, regionW = innerW + PADX * 2;
    const r = document.createElement("div"); r.className = "region " + node.role;
    r.style.left = ox + "px"; r.style.top = oy + "px"; r.style.width = regionW + "px"; r.style.height = regionH + "px";
    r.innerHTML = `<div class="rhead"><span class="nb" style="background:${node.role === "labs" ? "rgba(139,123,232,.13)" : "rgba(47,214,198,.13)"};">${disc(node.role === "labs" ? "session" : "k8s")}</span><div><div class="tt">${node.name}</div><div class="mm">${node.role}${node.ready ? " · Ready" : " · NotReady"}${node.kernel ? " · " + node.kernel : ""}</div></div></div>`;
    stage.insertBefore(r, stage.querySelector(".hive"));
    return { x: ox, y: oy, w: regionW, h: regionH, right: ox + regionW };
  }

  // Draw a lab session's nested k3s cluster as its own region: the nested
  // namespaces (only those with pods - kube-node-lease etc. are noise) as
  // hives, with the two isolation walls as substrate. Returns null when the
  // cluster isn't reachable/populated yet (kubeconfig still being fetched).
  function drawNested(session: AdminTopology["sessions"][number], oxIn: number, oy: number, kata: boolean, centerX?: number) {
    const nc = session.cluster;
    if (!nc || !nc.reachable) return null;
    // Show a nested namespace if it has pods OR carries escape-relevant content
    // (a Secret like production's flag, or a NetworkPolicy). Deliberately NOT
    // roles/roleBindings: every namespace ships stock built-in RBAC
    // (system:public-info-viewer etc.), so counting those rendered kube-public
    // / kube-node-lease / default as empty noise hives that drowned the actual
    // story (dev foothold -> production flag). The planted flaw here is a
    // cluster-scoped ClusterRoleBinding anyway, which no namespace tally shows.
    const notable = (r?: NsResources) => !!r && !!((r.secrets || 0) + (r.networkPolicies || 0));
    const nss = (nc.namespaces || []).filter((ns) => (ns.pods?.length ?? 0) > 0 || notable(ns.resources));
    if (!nss.length) return null;
    const hives: Hive[] = nss.map((ns) => ({ id: "nx/" + session.id + "/" + ns.name, name: ns.name, logo: ns.name === "kube-system" ? "k3s" : nsLogo(ns.name), pods: ns.pods, resources: ns.resources }));
    const COLGAP = 62, PADX = 32, TOPPAD = 52, HIVE_GAP = 44, PERCOL = 2;
    const cols: Hive[][] = [];
    hives.forEach((h, i) => { const c = Math.floor(i / PERCOL); (cols[c] ||= []).push(h); });
    const measured = cols.map((col) => col.map(measure));
    const colW = cols.map((_, ci) => Math.max(...measured[ci].map((m) => m.w), 158));
    // Compute the region width up front so the caller can centre the nested
    // cluster horizontally under the whole diagram (centerX), instead of it
    // hanging off the right node and marooning the bottom-left.
    const innerW = Math.max(colW.reduce((a, b) => a + b, 0) + COLGAP * (colW.length - 1), 520);
    const regionW = innerW + PADX * 2;
    const ox = centerX != null ? Math.round(centerX - regionW / 2) : oxIn;
    const colX: number[] = []; let cx = ox + PADX; colW.forEach((w, i) => { colX[i] = cx; cx += w + COLGAP; });
    const colBottom: number[] = [];
    cols.forEach((col, ci) => { let y = oy + TOPPAD; col.forEach((h, hi) => { const m = measured[ci][hi]; stage.appendChild(hiveEl(h, colX[ci] + (colW[ci] - m.w) / 2, y, m)); y += m.totalH + HIVE_GAP; }); colBottom[ci] = y - HIVE_GAP; });
    const subY = Math.max(...colBottom) + 50;
    // The isolation walls name the runtime ("runc - no microVM") and our
    // network architecture ("blocks outer API + backend"). Telling a student
    // which containment they are inside, and whether it is the weak local one,
    // is precisely the inner-tech reveal the scoped view exists to prevent.
    if (!scoped) {
      const sub = document.createElement("div"); sub.className = "subhdr"; sub.textContent = "isolation walls";
      sub.style.left = ox + PADX + "px"; sub.style.top = subY - 24 + "px"; sub.style.width = innerW + "px"; stage.appendChild(sub);
    }
    const walls = scoped ? [] : [
      { title: kata ? "Firecracker µVM" : "runc - no microVM (dev)", sub: "wall 1 · kernel", kind: kata ? "wall" : "warn" },
      { title: "session-runner NetworkPolicy", sub: "wall 2 · blocks outer API + backend", kind: "net" },
    ];
    let ly = subY;
    for (const w of walls) { const el = document.createElement("div"); el.className = "layer " + w.kind; el.style.left = ox + PADX + "px"; el.style.top = ly + "px"; el.style.width = innerW + "px"; el.innerHTML = `<span class="lt">${w.title}</span><span class="ls">${w.sub}</span>`; stage.appendChild(el); ly += 48; }
    const regionH = ly - 8 - oy + 44;
    const r = document.createElement("div"); r.className = "region nested";
    r.style.left = ox + "px"; r.style.top = oy + "px"; r.style.width = regionW + "px"; r.style.height = regionH + "px";
    const nestedLabel = scoped ? "your lab session" : `session ${session.id}`;
    r.innerHTML = `<div class="rhead"><span class="nb" style="background:rgba(139,123,232,.14)">${disc("k3s")}</span><div><div class="tt">nested k3s · ${nestedLabel}</div><div class="mm">${session.module || "lab"} · disposable target cluster</div></div></div>`;
    stage.insertBefore(r, stage.querySelector(".hive"));
    return { x: ox, y: oy, w: regionW, h: regionH, right: ox + regionW };
  }

  // layout nodes left to right
  let ox = 60; const oy = 60; const boxes: { x: number; y: number; w: number; h: number; right: number }[] = [];
  for (const node of rnodes) { const b = drawNode(node, ox, oy); boxes.push(b); ox = b.right + 210; }

  // ---- edges: resolve wiring keys to indexed pods (source=pod, dest=hive) ----
  // landOnHex: land an arrow on to.center as a mini-hex (like a pod), not on
  // the hive border - for a resource target (a secret) that isn't a pod.
  type Ref = { center: { cx: number; cy: number }; hiveCenter: { cx: number; cy: number }; hiveId?: string; pid?: string; landOnHex?: boolean };
  // Synthetic anchors for non-pod nodes the wiring can reference by key - today
  // just "cloudflare", registered by the access-lane block when prod draws the
  // gateway. Lets the static cloudflare->traefik edge resolve to a real point
  // instead of being dropped (it was skipped entirely before).
  const anchors: Record<string, Ref> = {};
  // resolve maps a wiring key to a live anchor. Beyond a plain pod-workload
  // match it also handles: session:<id> -> that session's runner pod (so the
  // backend's provisioning link draws to the runner, not nowhere); and a
  // kube-apiserver key that has no pod on k3s/k3d (API server runs in-process)
  // -> the control-plane kube-system hive, so backend->apiserver still draws
  // locally. scope, when given, restricts matches to one nested session's hives
  // (nx/<id>/...) so multiple concurrent sessions don't cross-anchor.
  function resolve(key: string, scope?: string): Ref | null {
    const k = key.toLowerCase();
    if (anchors[k]) return anchors[k];
    if (k.startsWith("session:")) {
      const id = k.slice(8);
      // The session's entry pod: a nested session's runner (labs-runners) or a
      // flat lab's pod (labs-baseline). Both name-carry the id. Without the
      // labs-baseline arm a flat lab's provisioning edge silently dropped, so
      // the session was invisible even though its pods render in the hive.
      const rp = podIndex.find((p) => p.hiveId.includes("/labs") && p.name.includes(id));
      return rp ? { center: rp.center, hiveCenter: rp.hiveCenter, hiveId: rp.hiveId, pid: rp.pid } : null;
    }
    if (k.startsWith("ns:")) {
      const ns = k.slice(3);
      const hid = Object.keys(pos).find((id) => (scope ? id.startsWith(scope) : !id.startsWith("nx/")) && id.endsWith("/" + ns));
      return hid ? { center: pos[hid], hiveCenter: pos[hid], hiveId: hid } : null;
    }
    // secret:<ns> - point at the namespace's secret hex (the thing the attack
    // reads) instead of the hive centre. Falls back to the hive if that
    // namespace happens to have no secret hex drawn.
    if (k.startsWith("secret:")) {
      const ns = k.slice(7);
      const hid = Object.keys(pos).find((id) => (scope ? id.startsWith(scope) : !id.startsWith("nx/")) && id.endsWith("/" + ns));
      if (!hid) return null;
      const sc = secretIndex[hid];
      return { center: sc ?? pos[hid], hiveCenter: pos[hid], hiveId: hid, landOnHex: sc !== undefined };
    }
    const pool = scope ? podIndex.filter((p) => p.hiveId.startsWith(scope)) : podIndex.filter((p) => !p.hiveId.startsWith("nx/"));
    let hit = pool.find((p) => p.key === k || p.name === k);
    if (!hit) hit = pool.find((p) => p.key.startsWith(k) || k.startsWith(p.key));
    if (!hit) hit = pool.find((p) => p.key.includes(k));
    if (hit) return { center: hit.center, hiveCenter: hit.hiveCenter, hiveId: hit.hiveId, pid: hit.pid };
    if (k.includes("apiserver")) {
      const hid = Object.keys(pos).find((id) => id.endsWith("/kube-system") && !id.startsWith("nx/"));
      if (hid) return { center: pos[hid], hiveCenter: pos[hid], hiveId: hid };
    }
    return null;
  }
  // hexBoundary returns the point on a hive's hexagon border along the ray from
  // its centre toward `toward`, pushed `gap` px further out - so an arrow tip
  // rests just outside the stroke instead of diving into the hexagon. The
  // vertex layout matches the bighex clip-path (flat top/bottom, pointed sides).
  function hexBoundary(g: { cx: number; cy: number; halfW: number; halfH: number }, toward: { cx: number; cy: number }, gap: number) {
    const dx = toward.cx - g.cx, dy = toward.cy - g.cy, L = Math.hypot(dx, dy) || 1;
    const hw = g.halfW, hh = g.halfH;
    const V: [number, number][] = [[-0.5 * hw, -hh], [0.5 * hw, -hh], [hw, 0], [0.5 * hw, hh], [-0.5 * hw, hh], [-hw, 0]];
    let best = Infinity;
    for (let i = 0; i < 6; i++) {
      const p1 = V[i], p2 = V[(i + 1) % 6];
      const ex = p2[0] - p1[0], ey = p2[1] - p1[1];
      const det = -dx * ey + ex * dy;
      if (Math.abs(det) < 1e-9) continue;
      const t = (-p1[0] * ey + ex * p1[1]) / det;
      const u = (dx * p1[1] - dy * p1[0]) / det;
      if (t > 0 && u >= -0.001 && u <= 1.001 && t < best) best = t;
    }
    if (!isFinite(best)) return { cx: g.cx, cy: g.cy };
    return { cx: g.cx + dx * best + (dx / L) * gap, cy: g.cy + dy * best + (dy / L) * gap };
  }
  // insideHive: is (px,py) within a hive's hexagon? At height |y| the hex spans
  // ±halfW*(1 - 0.5*|y|/halfH) (matches the bighex vertex slope).
  function insideHive(px: number, py: number, g: { cx: number; cy: number; halfW: number; halfH: number }) {
    const x = Math.abs(px - g.cx), y = Math.abs(py - g.cy);
    return y <= g.halfH && x <= g.halfW * (1 - 0.5 * (y / g.halfH));
  }
  // routeBow picks the curve control point that makes the edge cross the fewest
  // OTHER hives (its own endpoints' hives are allowed). It tries both sides at
  // growing magnitudes and samples the quadratic against every hexagon; least
  // hits wins, ties break toward the gentler curve and the preferred side.
  const allGeom = () => Object.entries(hiveGeom);
  function routeBow(a: { cx: number; cy: number }, b: { cx: number; cy: number }, exclude: Set<string>, baseK: number, prefer: number) {
    const dx = b.cx - a.cx, dy = b.cy - a.cy, L = Math.hypot(dx, dy) || 1;
    const nx = -dy / L, ny = dx / L;
    const obstacles = allGeom().filter(([id]) => !exclude.has(id)).map(([, g]) => g);
    const score = (k: number) => {
      const cx = (a.cx + b.cx) / 2 + nx * k, cy = (a.cy + b.cy) / 2 + ny * k;
      let hits = 0;
      for (let s = 1; s <= 8; s++) {
        const t = s / 9, it = 1 - t;
        const px = it * it * a.cx + 2 * it * t * cx + t * t * b.cx;
        const py = it * it * a.cy + 2 * it * t * cy + t * t * b.cy;
        for (const g of obstacles) { if (insideHive(px, py, g)) { hits++; break; } }
      }
      return { cx, cy, hits, k };
    };
    const mags = [1, 1.7, 2.5, 3.4].flatMap((m) => [prefer * m, -prefer * m]);
    let best = score(baseK * mags[0]);
    for (const m of mags.slice(1)) {
      const r = score(baseK * m);
      if (r.hits < best.hits || (r.hits === best.hits && Math.abs(r.k) < Math.abs(best.k))) best = r;
      if (best.hits === 0) break;
    }
    return best;
  }
  const edgeRefs: { p: SVGPathElement; t: SVGTextElement | null; fromHive: string; toHive: string; fromPid?: string; toPid?: string }[] = [];
  function inset(pt: { cx: number; cy: number }, toward: { cx: number; cy: number }, by: number) {
    const dx = toward.cx - pt.cx, dy = toward.cy - pt.cy, d = Math.max(1, Math.hypot(dx, dy));
    return { cx: pt.cx + (dx / d) * by, cy: pt.cy + (dy / d) * by };
  }
  // emitEdge draws one directed link + optional label. Styling by kind: prov
  // (violet), escape (red, animated - the attack path), observes (teal, eye
  // instead of an arrowhead - a sensor watching, not traffic), everything else
  // grey.
  function emitEdge(from: Ref, to: Ref, port: string, kind: string, bow = 1): { p: SVGPathElement; t: SVGTextElement | null } {
    // "in" = an ingress/access hop that crosses the cluster boundary (public
    // internet or host SSH), coloured distinctly from grey in-cluster traffic.
    const ext = kind === "in";
    const obs = kind === "observes";
    const col = kind === "prov" ? "var(--tp-violet)" : kind === "escape" ? "#ff5b6b" : obs ? "var(--tp-accent)" : ext ? "var(--tp-ext)" : "var(--tp-ink3)";
    const marker = kind === "prov" ? "url(#tp-av)" : kind === "escape" ? "url(#tp-ae)" : obs ? "url(#tp-eye)" : ext ? "url(#tp-aext)" : "url(#tp-a)";
    let a: { cx: number; cy: number }, b: { cx: number; cy: number }, cx: number, cy: number;
    // A self-referential edge: source and destination are the SAME pod. That is
    // exactly what a LOCAL privilege escalation is - the pod becomes root in
    // place, with nowhere to travel to (suid-privesc, ebpf-detect). The
    // normal paths can't express it: a pod-to-pod arch degenerates to a
    // zero-length curve, and aiming at the enclosing namespace lands the arrow
    // on whatever resource hex sits nearest the hive centre. So draw a loop that
    // leaves the pod and returns to it, bowed away from the hive centre so it
    // never crosses the pod's siblings. Only a, b and the control point differ;
    // stroke, marker and label are the shared code below.
    const selfLoop = !!(from.pid && to.pid && from.pid === to.pid);
    const sameHive = !selfLoop && !!(from.hiveId && to.hiveId && from.hiveId === to.hiveId && hiveGeom[from.hiveId]);
    if (selfLoop) {
      const c = from.center;
      const g = from.hiveId ? hiveGeom[from.hiveId] : undefined;
      // Bow outward, away from the hive centre - or straight up if this pod IS
      // the centre (a lone pod in its namespace).
      let ox = 0, oy = -1;
      if (g) {
        const dx = c.cx - g.cx, dy = c.cy - g.cy, L = Math.hypot(dx, dy);
        if (L > 2) { ox = dx / L; oy = dy / L; }
      }
      const ux = -oy, uy = ox; // perpendicular: the loop's two feet on the pod
      const foot = MINw * 0.24, rim = MINh / 2;
      a = { cx: c.cx + ox * rim * 0.5 + ux * foot, cy: c.cy + oy * rim * 0.5 + uy * foot };
      b = { cx: c.cx + ox * rim * 0.5 - ux * foot, cy: c.cy + oy * rim * 0.5 - uy * foot };
      // Control point chosen so the quadratic passes through the apex at t=0.5,
      // which is also where the shared label code puts the text.
      const apex = { cx: c.cx + ox * (rim + 34), cy: c.cy + oy * (rim + 34) };
      cx = apex.cx * 2 - (a.cx + b.cx) / 2;
      cy = apex.cy * 2 - (a.cy + b.cy) / 2;
    } else if (sameHive) {
      // Intra-hive link (e.g. observability's alloy->loki->grafana): both pods
      // sit in the same hexagon. Draw a big pod-to-pod arch that bows OUTWARD
      // far enough that its apex - where the label rides - clears the hive's
      // hexagon border, so ":3100" ends up in open space outside the hive
      // rather than on top of a neighbouring pod.
      const g = hiveGeom[from.hiveId!];
      a = from.center; const b0 = to.center; // b0 = target pod centre
      const dx = b0.cx - a.cx, dy = b0.cy - a.cy, L = Math.hypot(dx, dy) || 1;
      const mx = (a.cx + b0.cx) / 2, my = (a.cy + b0.cy) / 2;
      let px = -dy / L, py = dx / L; // perpendicular unit vector
      // Bow DOWNWARD by preference: the namespace header always sits just above
      // the hex, so an upward arch drops its label (":5432") into that cramped
      // ~10px header band. Point the perpendicular down. Only when it's
      // near-horizontal (a vertical chord, so there's no meaningful "down") fall
      // back to bowing outward from the hive centre for a left/right apex.
      if (Math.abs(py) > 0.25) {
        if (py < 0) { px = -px; py = -py; }
      } else {
        let outx = mx - g.cx, outy = my - g.cy;
        if (Math.hypot(outx, outy) < 2) { outx = px; outy = py; }
        if (px * outx + py * outy < 0) { px = -px; py = -py; }
      }
      // Bezier apex sits at mid + perp*half; size `half` so it reaches ~15px
      // past the hexagon border in the outward direction (label rides outside).
      const bp = hexBoundary(g, { cx: g.cx + px * 1000, cy: g.cy + py * 1000 }, 15);
      const half = Math.max(30, (bp.cx - mx) * px + (bp.cy - my) * py);
      cx = mx + px * half * 2; cy = my + py * half * 2;
      // Stop the arrow at the target pod's own hex edge (approaching from the
      // apex side), not buried in its centre.
      b = hexBoundary({ cx: b0.cx, cy: b0.cy, halfW: MINw / 2, halfH: MINh / 2 }, { cx, cy }, 2);
    } else {
      // Destination: a pod-specific target (to.pid set) lands the arrow on that
      // pod's own mini-hex, so an edge to e.g. the backend or traefik reaches
      // the exact pod - not the whole namespace border. Group targets with no
      // pod (the apiserver fallback, ns: escape targets) still land on the hive
      // border. Source: start just clear of the origin pod hex.
      const dg = to.hiveId ? hiveGeom[to.hiveId] : undefined;
      // rimSide: the point on a pod's mini-hex facing away from its OWN hive
      // centre, i.e. toward the nearest hive perimeter. Entering/leaving there
      // lets an edge round a hive instead of slicing across its pods. Returns
      // null (caller falls back to a direct anchor) when the pod sits near the
      // hive centre, OR when that outward side points sharply AWAY from the
      // other endpoint - forcing it there would make the edge loop the long way
      // round rather than approach cleanly, which is worse than a short slice.
      const rimSide = (center: { cx: number; cy: number }, hiveId: string | undefined, gap: number, toward: { cx: number; cy: number }) => {
        const g = hiveId ? hiveGeom[hiveId] : undefined;
        if (!g) return null;
        const dx = center.cx - g.cx, dy = center.cy - g.cy, L = Math.hypot(dx, dy);
        if (L <= MINh * 0.5) return null;
        const ux = dx / L, uy = dy / L;
        const tx = toward.cx - center.cx, ty = toward.cy - center.cy, tL = Math.hypot(tx, ty) || 1;
        if ((ux * tx + uy * ty) / tL < -0.35) return null; // outward points away from the other end
        return hexBoundary({ cx: center.cx, cy: center.cy, halfW: MINw / 2, halfH: MINh / 2 }, { cx: center.cx + ux * 500, cy: center.cy + uy * 500 }, gap);
      };
      // Destination: land on the target pod's rim side (facing the near
      // perimeter) so the edge enters from just outside - typically from below
      // for a low pod - not by cutting across the siblings between the hive edge
      // and the pod. Hive-level targets (apiserver fallback, ns: escapes) still
      // land on the hive border facing the source.
      // bRimHit: b is a precise landing on the target (a pod's rim, or a
      // secret hex), not a hive-border fallback. Drives the hive exclusion
      // below - a precise target inside a hive must NOT exclude that hive, or
      // the router would route around it and the arrow would never reach in.
      let bRimHit = false;
      if (to.landOnHex) {
        // A secret target: land the arrowhead ON the secret hex, entering from
        // the source side, so it visibly touches the secret rather than
        // stopping short in the hive. rimSide (built to route AROUND a hive to
        // a pod) lands on the far/outward face and leaves a gap - wrong here,
        // where the point is to reach the thing inside. Poke ~3px past the rim
        // toward centre so the tip overlaps the hex edge.
        const dx = from.center.cx - to.center.cx, dy = from.center.cy - to.center.cy;
        const L = Math.hypot(dx, dy) || 1;
        const reach = MINh / 2 - 3;
        b = { cx: to.center.cx + (dx / L) * reach, cy: to.center.cy + (dy / L) * reach };
        bRimHit = true;
      } else {
        const bRim = to.pid ? rimSide(to.center, to.hiveId, 2, from.center) : null;
        bRimHit = !!bRim;
        b = bRim
          || (to.pid ? hexBoundary({ cx: to.center.cx, cy: to.center.cy, halfW: MINw / 2, halfH: MINh / 2 }, from.center, 2)
            : dg ? hexBoundary(dg, from.center, 2) : inset(to.hiveCenter, from.center, 42));
      }
      // Source: leave the origin pod from ITS rim side too, so the edge exits
      // its own hive at the near perimeter instead of doubling back across it.
      const aRim = from.pid ? rimSide(from.center, from.hiveId, 3, b) : null;
      a = aRim || inset(from.center, b, 13);
      // With both anchors on the hives' outer rims, routeBow can keep the curve
      // OUTSIDE both hexagons - so an edge rounds the hives rather than driving
      // through them. Only exclude an endpoint hive when its anchor sits on the
      // rim (excluding it would let the router ignore - and re-cross - it);
      // hive-level targets keep the old exclusion so they can enter the hive.
      const L = Math.hypot(b.cx - a.cx, b.cy - a.cy) || 1;
      const exclude = new Set<string>();
      if (from.hiveId && !aRim) exclude.add(from.hiveId);
      if (to.hiveId && !bRimHit) exclude.add(to.hiveId);
      ({ cx, cy } = routeBow(a, b, exclude, Math.min(L * 0.16, 66), bow));
    }
    const path = document.createElementNS(NS, "path");
    path.setAttribute("d", `M${a.cx} ${a.cy} Q ${cx} ${cy} ${b.cx} ${b.cy}`);
    path.setAttribute("fill", "none"); path.setAttribute("stroke", col); path.setAttribute("stroke-width", kind === "escape" ? "1.7" : ext ? "1.6" : obs ? "1.3" : "1.4");
    // A finer, tighter dash for observation: it should read as a passive gaze
    // sitting behind the diagram, not as something flowing like the other links.
    path.setAttribute("stroke-dasharray", obs ? "2 4" : "5 5"); path.setAttribute("marker-end", marker);
    (path as unknown as HTMLElement).style.transition = "opacity .15s";
    if (kind === "escape") {
      const an = document.createElementNS(NS, "animate"); an.setAttribute("attributeName", "stroke-dashoffset"); an.setAttribute("from", "20"); an.setAttribute("to", "0"); an.setAttribute("dur", ".7s"); an.setAttribute("repeatCount", "indefinite"); path.appendChild(an);
    }
    edges.appendChild(path);
    let t: SVGTextElement | null = null;
    if (port) {
      // Label at the quadratic's apex (t=0.5) - the bow puts it in clear space.
      const lx = 0.25 * a.cx + 0.5 * cx + 0.25 * b.cx, ly = 0.25 * a.cy + 0.5 * cy + 0.25 * b.cy;
      t = document.createElementNS(NS, "text");
      t.setAttribute("x", String(lx)); t.setAttribute("y", String(ly - 3)); t.setAttribute("text-anchor", "middle");
      t.setAttribute("fill", kind === "escape" ? "#ff8a94" : ext ? "var(--tp-ext)" : "var(--tp-ink2)"); t.setAttribute("font-size", "10.5"); t.textContent = port;
      (t as unknown as HTMLElement).style.transition = "opacity .15s"; edges.appendChild(t);
    }
    return { p: path, t };
  }
  // ---- access lane: who reaches the platform and how, grounded in the real
  // ingress path. Prod sits behind the Cloudflare proxy (origin firewalled to
  // Cloudflare-only), so user + admin enter through Cloudflare -> Traefik, and
  // admins additionally SSH to the nodes from an allowlisted IP (hostfw /
  // WireGuard, kubelet CP-only). Local dev has no proxy: you hit the dev
  // servers directly (frontend :7373, backend :8080). These context edges stay
  // always-on (not added to edgeRefs) so the "who gets in" story never dims.
  //
  // Suppressed entirely for a student: this lane names an admin actor reaching
  // the platform from an allowlisted IP over SSH, which describes our
  // administrative access path and has nothing to do with their lab.
  if (!scoped) {
    // Actors sit centred over the whole diagram (not left-aligned) and up near
    // the top, so the "who gets in" row reads as a header. Cloudflare centres
    // directly beneath them so the user + admin edges drop straight down into
    // it, instead of one having to cut back across to a left-hung gateway.
    // Actors sit well above the host regions, with the Cloudflare gateway in
    // the band between - spaced generously so the "who gets in" row reads as a
    // header with air around it, not crammed against the region tops (the SSH /
    // :443 edge labels ride the gaps, so tight spacing bunched them up).
    const laneY = oy - 264, gwY = oy - 128;
    const midX = boxes.length ? (boxes[0].x + boxes[boxes.length - 1].right) / 2 : 300;
    type Actor = { el: HTMLElement; w: number; h: number; center: { cx: number; cy: number }; bottom: { cx: number; cy: number }; top: number };
    // Build offscreen so we can measure width before deciding the centred x.
    const mkActor = (key: string, name: string, sub: string, ext = false): Actor => {
      const el = document.createElement("div");
      el.className = "actor" + (ext ? " ext" : "");
      el.style.left = "-9999px"; el.style.top = laneY + "px";
      el.innerHTML = `${disc(key)}<div class="atx"><div class="an">${name}</div><div class="as">${sub}</div></div>`;
      stage.appendChild(el);
      return { el, w: el.offsetWidth, h: el.offsetHeight, center: { cx: 0, cy: 0 }, bottom: { cx: 0, cy: 0 }, top: laneY };
    };
    const place = (a: Actor, x: number, y: number) => {
      a.el.style.left = x + "px"; a.el.style.top = y + "px"; a.top = y;
      a.center = { cx: x + a.w / 2, cy: y + a.h / 2 }; a.bottom = { cx: x + a.w / 2, cy: y + a.h };
    };
    const ref = (pt: { cx: number; cy: number }) => ({ center: pt, hiveCenter: pt });
    const userA = mkActor("user", "user", "public app");
    const adminA = mkActor("admin", "admin", "allowlisted IP");
    const AGAP = 96, groupW = userA.w + AGAP + adminA.w, gx = Math.round(midX - groupW / 2);
    place(userA, gx, laneY);
    place(adminA, gx + userA.w + AGAP, laneY);

    if (kata) {
      const cf = mkActor("cloudflare", "Cloudflare", "proxy · TLS · WAF", true);
      place(cf, Math.round(midX - cf.w / 2), gwY); // centred directly under the actors
      anchors["cloudflare"] = ref(cf.center); // lets cloudflare->traefik (wiring) draw
      emitEdge(ref(userA.bottom), ref(cf.center), ":443 · app", "in", 1);
      emitEdge(ref(adminA.bottom), ref(cf.center), ":443 · admin · allowlist", "in", -1);
      // admin SSH goes straight to each node host, bypassing the proxy. Both
      // the control-plane and the labs node accept :22 only from the admin IP
      // (Terraform cp firewall + the AX41 nftables hostfw), so draw one SSH
      // edge per node - otherwise the labs node reads as unreachable until a
      // lab session provisions a runner onto it.
      boxes.forEach((nd, i) => {
        const a = adminA.bottom, tgt = { cx: nd.x + nd.w * (i === 0 ? 0.72 : 0.3), cy: nd.y };
        // Cloudflare sits centred just below the actors, so bow each SSH edge
        // outward (away from centre) to skirt it rather than cross it, and drop
        // the label into the clear band below Cloudflare.
        const side = tgt.cx < midX ? -1 : 1, my = (a.cy + tgt.cy) / 2;
        const p = document.createElementNS(NS, "path");
        p.setAttribute("d", `M${a.cx} ${a.cy} C ${a.cx + side * 72} ${my}, ${tgt.cx + side * 20} ${my}, ${tgt.cx} ${tgt.cy - 4}`);
        p.setAttribute("fill", "none"); p.setAttribute("stroke", "var(--tp-ext)"); p.setAttribute("stroke-width", "1.6"); p.setAttribute("stroke-dasharray", "2 4"); p.setAttribute("marker-end", "url(#tp-aext)");
        edges.appendChild(p);
        const t = document.createElementNS(NS, "text"); t.setAttribute("x", String((a.cx + tgt.cx) / 2 + side * 46)); t.setAttribute("y", String(a.cy + (tgt.cy - a.cy) * 0.8)); t.setAttribute("text-anchor", "middle"); t.setAttribute("fill", "var(--tp-ext)"); t.setAttribute("font-size", "10.5"); t.textContent = i === 0 ? "SSH · allowlisted IP" : "SSH"; edges.appendChild(t);
      });

      // admin -> kube-apiserver :6443 (kubectl). Distinct from every other
      // path on this map and previously missing from it, which mattered: this
      // is cluster-admin, and it bypasses the whole application stack - none of
      // requireAdmin, the entitlement checks, or the Traefik admin allowlist
      // sit anywhere on it. A map that showed the app's gates but not this one
      // understated who can reach what.
      //
      // Only to the control-plane node: the API server listens there alone,
      // unlike SSH which reaches both hosts. Port 6443 is the external
      // listener - the backend's own client-go edge in wiring.go is :443
      // because in-cluster it goes through the kubernetes.default.svc Service.
      const cpIdx = rnodes.findIndex((n) => n.role === "control-plane");
      if (cpIdx >= 0 && boxes[cpIdx]) {
        const nd = boxes[cpIdx], a = adminA.bottom;
        // Land on the node's top-left, clear of the SSH edge which aims at
        // 0.72 of its width.
        const tgt = { cx: nd.x + nd.w * 0.12, cy: nd.y };
        const my = (a.cy + tgt.cy) / 2;
        const p = document.createElementNS(NS, "path");
        p.setAttribute("d", `M${a.cx} ${a.cy} C ${a.cx - 150} ${my}, ${tgt.cx - 40} ${my}, ${tgt.cx} ${tgt.cy - 4}`);
        p.setAttribute("fill", "none"); p.setAttribute("stroke", "var(--tp-ext)"); p.setAttribute("stroke-width", "1.6"); p.setAttribute("stroke-dasharray", "2 4"); p.setAttribute("marker-end", "url(#tp-aext)");
        edges.appendChild(p);
        const t = document.createElementNS(NS, "text");
        t.setAttribute("x", String(tgt.cx - 26)); t.setAttribute("y", String(a.cy + (tgt.cy - a.cy) * 0.62));
        t.setAttribute("text-anchor", "middle"); t.setAttribute("fill", "var(--tp-ext)"); t.setAttribute("font-size", "10.5");
        t.textContent = "kubectl :6443 · allowlisted IP";
        edges.appendChild(t);
      }
    } else {
      // Local: no Cloudflare, but the cluster ingress (Traefik) is still the
      // front door - it routes / to the frontend and /api,/admin,/ui to the
      // backend (the traefik->frontend/backend wiring edges carry it onward).
      // So point the actors at Traefik, not straight at the backend, matching
      // the real request path.
      const tf = resolve("traefik");
      if (tf) {
        emitEdge(ref(userA.bottom), tf, "app", "in", 1);
        emitEdge(ref(adminA.bottom), tf, "admin · allowlist", "in", -1);
      }
    }
  }

  data.edges.forEach((e, i) => {
    // cloudflare edges resolve only in prod (the access lane registers the
    // gateway anchor); locally resolve() returns null and the edge is dropped.
    const from = resolve(e.from), to = resolve(e.to);
    if (!from || !to) return;
    const { p: path, t } = emitEdge(from, to, e.port || "", e.kind, i % 2 === 0 ? 1 : -1);
    const fh = from.hiveId || podIndex.find((p) => p.center === from.center)?.hiveId || "";
    const th = to.hiveId || podIndex.find((p) => p.center === to.center)?.hiveId || "";
    edgeRefs.push({ p: path, t, fromHive: fh, toHive: th, fromPid: from.pid, toPid: to.pid });
  });

  // Admin dashboards (Grafana/ArgoCD/...) aren't reached directly - the backend
  // reverse-proxies them under /admin/* (see internal/api/adminproxy.go). Draw
  // that hop backend -> dashboard pod so the admin UIs sit on the access path,
  // not just as ↗ links. Skip swagger (served by the backend itself) and vault
  // (already has a backend edge in the wiring) to avoid a self/duplicate link.
  {
    const be = resolve("backend"), skip = new Set(["backend", "vault"]);
    if (be) for (const d of data.dashboards || []) {
      if (skip.has(d.ref)) continue;
      const t = resolve(d.ref);
      if (!t) continue;
      const { p, t: lbl } = emitEdge(be, t, d.path, "podapp", 1);
      edgeRefs.push({ p, t: lbl, fromHive: be.hiveId || "", toHive: t.hiveId || "", fromPid: be.pid, toPid: t.pid });
    }
  }

  // ---- nested lab clusters: draw each session's nested k3s below the labs node ----
  {
    // Which hive the "runs here" connector hangs off. Admins get the real
    // runner namespace. A student has no labs-runners hive - the outer cluster
    // is deliberately reduced to the opaque boundary - so anchor on that
    // instead, or the nested region draws with nothing joining it to anything
    // and the boundary node just floats.
    const lrKey = scoped
      ? Object.keys(pos)[0]
      : Object.keys(pos).find((k) => k.endsWith("/labs-runners"));
    // Centre the nested cluster(s) horizontally under the whole diagram and
    // tuck them just below the tallest node region, so they read as the shared
    // child of the platform rather than marooning the bottom-left corner.
    const centerX = boxes.length ? (boxes[0].x + boxes[boxes.length - 1].right) / 2 : 60;
    const tallest = boxes.reduce((m, b) => Math.max(m, b.y + b.h), oy);
    let ny = tallest + 48;
    for (const s of data.sessions || []) {
      const nb = drawNested(s, 60, ny, kata, centerX);
      if (!nb) continue;
      if (lrKey && pos[lrKey]) {
        const a = pos[lrKey], b = { cx: nb.x + nb.w / 2, cy: nb.y };
        const my = (a.cy + b.cy) / 2;
        const p = document.createElementNS(NS, "path");
        p.setAttribute("d", `M${a.cx} ${a.cy} C ${a.cx} ${my}, ${b.cx} ${my}, ${b.cx} ${b.cy - 6}`);
        p.setAttribute("fill", "none"); p.setAttribute("stroke", "var(--tp-violet)"); p.setAttribute("stroke-width", "1.6");
        p.setAttribute("stroke-dasharray", "5 5"); p.setAttribute("marker-end", "url(#tp-av)");
        const an = document.createElementNS(NS, "animate"); an.setAttribute("attributeName", "stroke-dashoffset"); an.setAttribute("from", "20"); an.setAttribute("to", "0"); an.setAttribute("dur", ".8s"); an.setAttribute("repeatCount", "indefinite"); p.appendChild(an);
        edges.appendChild(p);
        const t = document.createElementNS(NS, "text"); t.setAttribute("x", String((a.cx + b.cx) / 2)); t.setAttribute("y", String(my - 6));
        t.setAttribute("text-anchor", "middle"); t.setAttribute("fill", "var(--tp-violet)"); t.setAttribute("font-size", "10.5");
        // "provisions · nested k3s" is platform vocabulary. A student is told
        // whose cluster it is, not how we made it.
        t.textContent = scoped ? "your lab runs here" : "provisions · nested k3s";
        edges.appendChild(t);
      }
      // Nested-internal edges (the escape path: foothold -> the Secret it should
      // never reach). Resolved scoped to THIS session's hives (nx/<id>/...) so
      // concurrent sessions can't cross-anchor onto each other's foothold.
      const scope = "nx/" + s.id + "/";
      for (const ne of s.cluster?.edges || []) {
        const from = resolve(ne.from, scope), to = resolve(ne.to, scope);
        if (from && to) {
          const { p: path, t } = emitEdge(from, to, ne.port || "", ne.kind || "escape", -1);
          edgeRefs.push({ p: path, t, fromHive: from.hiveId || "", toHive: to.hiveId || "", fromPid: from.pid, toPid: to.pid });
        }
      }
      // The user's interactive shell. The backend brokers it - ttyd for legacy
      // pod sessions, an exec stream into the nested entry pod for nested ones
      // (internal/labs/terminal.go) - so it originates at the backend, not the
      // user, who reaches the backend over the ingress path already drawn and
      // never touches the isolated pod directly. Target the entry pod (the
      // escape edge's source), falling back to the runner if a module has no
      // escape edge. Coloured as host/interactive access (blue), like SSH.
      {
        const be = resolve("backend");
        const entryKey = (s.cluster?.edges || []).find((e) => (e.kind || "escape") === "escape")?.from;
        const shellTo = (entryKey && resolve(entryKey, scope)) || resolve("session:" + s.id);
        if (be && shellTo) {
          const { p: path, t } = emitEdge(be, shellTo, "shell · ttyd", "in", 1);
          edgeRefs.push({ p: path, t, fromHive: be.hiveId || "", toHive: shellTo.hiveId || "", fromPid: be.pid, toPid: shellTo.pid });
        }
      }
      ny = nb.y + nb.h + 56;
    }
  }

  // ---- label de-confliction: nudge edge labels off anything they'd sit on ----
  // routeBow steers each CURVE around other hives, but the label rides the apex
  // and can still land on a hexagon, an actor card, or a neighbouring label -
  // exactly the overlaps the map showed. This is a short relaxation pass: every
  // <text> is a point mass repelled by hives/actors and by other labels, on a
  // soft spring back to its home apex and hard-clamped to a small travel radius,
  // so a label always stays beside its own edge - just shoved into the nearest
  // gap. Runs in stage coordinates (same space the labels + hiveGeom live in),
  // independent of the camera transform, so it's computed once at draw time.
  function deconflictLabels() {
    type Box = { cx: number; cy: number; hw: number; hh: number };
    type Lab = Box & { t: SVGTextElement; hx: number; hy: number };
    const texts = Array.from(edges.querySelectorAll("text")) as SVGTextElement[];
    if (texts.length < 2) return;
    const labs: Lab[] = texts.map((t) => {
      const x = parseFloat(t.getAttribute("x") || "0"), yb = parseFloat(t.getAttribute("y") || "0");
      let len = 40;
      try { len = t.getComputedTextLength() || len; } catch { /* not measurable yet - keep the estimate */ }
      // The y attribute is the baseline; the visual centre sits ~4px above it,
      // and 10.5px text is ~15px tall -> half-height ~7.5.
      return { t, cx: x, cy: yb - 4, hw: len / 2 + 4, hh: 7.5, hx: x, hy: yb - 4 };
    });
    const obstacles: Box[] = [];
    for (const g of Object.values(hiveGeom)) obstacles.push({ cx: g.cx, cy: g.cy, hw: g.halfW, hh: g.halfH });
    stage.querySelectorAll<HTMLElement>(".actor").forEach((el) => {
      const l = parseFloat(el.style.left || "0"), tp = parseFloat(el.style.top || "0");
      if (l < -9000) return; // an actor still parked offscreen for width measurement
      obstacles.push({ cx: l + el.offsetWidth / 2, cy: tp + el.offsetHeight / 2, hw: el.offsetWidth / 2, hh: el.offsetHeight / 2 });
    });
    // Namespace headers (icon + name + ↗) sit ABOVE each hex, outside hiveGeom,
    // so a label could still land on one - the ":5432"-on-"default" case. Add
    // each header's content box as an obstacle, measured off the live layout.
    // The stage transform isn't applied until fit() runs later, so client rects
    // are 1:1 with stage coords here (same space the text x/y attributes use).
    const sr = stage.getBoundingClientRect();
    stage.querySelectorAll<HTMLElement>(".hive .hhead").forEach((head) => {
      const disc = head.querySelector(".disc"), nm = head.querySelector(".hn"), dh = head.querySelector(".hdash");
      if (!disc || !nm) return;
      const hr = head.getBoundingClientRect();
      const left = disc.getBoundingClientRect().left, right = (dh || nm).getBoundingClientRect().right;
      const l = left - sr.left, r = right - sr.left, t = hr.top - sr.top, b = hr.bottom - sr.top;
      obstacles.push({ cx: (l + r) / 2, cy: (t + b) / 2, hw: (r - l) / 2 + 3, hh: (b - t) / 2 });
    });
    // Signed 1-D overlap of two centred spans: >0 means they intersect on that
    // axis. Two boxes collide only when BOTH axes overlap; the smaller overlap
    // is the cheaper escape direction, so we push along it.
    const ov = (a: number, ah: number, b: number, bh: number) => ah + bh - Math.abs(a - b);
    const MAX = 58; // px a label may drift from its apex before it's clamped back
    for (let iter = 0; iter < 36; iter++) {
      for (const lb of labs) {
        let fx = 0, fy = 0, hit = false;
        for (const o of obstacles) {
          const ox = ov(lb.cx, lb.hw, o.cx, o.hw), oy = ov(lb.cy, lb.hh, o.cy, o.hh);
          if (ox > 0 && oy > 0) { hit = true; if (ox < oy) fx += lb.cx < o.cx ? -ox : ox; else fy += lb.cy < o.cy ? -oy : oy; }
        }
        for (const ob of labs) {
          if (ob === lb) continue;
          const ox = ov(lb.cx, lb.hw, ob.cx, ob.hw), oy = ov(lb.cy, lb.hh, ob.cy, ob.hh);
          if (ox > 0 && oy > 0) { hit = true; if (ox < oy) fx += (lb.cx < ob.cx ? -ox : ox) * 0.5; else fy += (lb.cy < ob.cy ? -oy : oy) * 0.5; }
        }
        // Spring home only pulls when the label is already CLEAR - otherwise the
        // pull balances the repel at a small residual penetration (the ~4px
        // sliver the argocd label showed). When overlapping, repel unopposed so
        // it fully escapes; once free, the (gentle) spring re-centres it.
        if (!hit) { fx += (lb.hx - lb.cx) * 0.12; fy += (lb.hy - lb.cy) * 0.12; }
        lb.cx += Math.max(-6, Math.min(6, fx * 0.5));
        lb.cy += Math.max(-6, Math.min(6, fy * 0.5));
        const dx = lb.cx - lb.hx, dy = lb.cy - lb.hy, d = Math.hypot(dx, dy);
        if (d > MAX) { lb.cx = lb.hx + (dx / d) * MAX; lb.cy = lb.hy + (dy / d) * MAX; }
      }
    }
    for (const lb of labs) { lb.t.setAttribute("x", String(Math.round(lb.cx))); lb.t.setAttribute("y", String(Math.round(lb.cy + 4))); }
  }
  deconflictLabels();

  const setEdge = (r: (typeof edgeRefs)[number], on: boolean) => { r.p.style.opacity = on ? "1" : ".08"; if (r.t) (r.t as unknown as HTMLElement).style.opacity = on ? "1" : "0"; };
  const clearPodDim = () => stage.querySelectorAll<HTMLElement>(".mini.dim").forEach((m) => m.classList.remove("dim"));
  // Hive-level focus: emphasise the hovered namespace + everything it links to.
  function highlight(id: string | null) {
    clearPodDim();
    const hives = stage.querySelectorAll<HTMLElement>(".hive");
    if (!id) { hives.forEach((h) => h.classList.remove("dim")); edgeRefs.forEach((r) => setEdge(r, true)); return; }
    const linked = new Set([id]); edgeRefs.forEach((r) => { if (r.fromHive === id || r.toHive === id) { linked.add(r.fromHive); linked.add(r.toHive); } });
    hives.forEach((h) => h.classList.toggle("dim", !linked.has(h.dataset.id!)));
    edgeRefs.forEach((r) => setEdge(r, r.fromHive === id || r.toHive === id));
  }
  // Pod-level focus: hovering a single pod isolates only that pod's own links
  // (and the pods/hives on the other end), dimming its siblings too - so you
  // read exactly what that one component talks to, not its whole namespace.
  function highlightPod(pid: string) {
    const ownHive = pid.slice(0, pid.lastIndexOf("#"));
    const pods = new Set([pid]), hivesOn = new Set([ownHive]);
    edgeRefs.forEach((r) => {
      if (r.fromPid !== pid && r.toPid !== pid) return;
      if (r.fromPid) pods.add(r.fromPid); if (r.toPid) pods.add(r.toPid);
      hivesOn.add(r.fromHive); hivesOn.add(r.toHive);
    });
    stage.querySelectorAll<HTMLElement>(".hive").forEach((h) => h.classList.toggle("dim", !hivesOn.has(h.dataset.id!)));
    stage.querySelectorAll<HTMLElement>(".mini[data-pod]").forEach((m) => m.classList.toggle("dim", !pods.has(m.dataset.pod!)));
    edgeRefs.forEach((r) => setEdge(r, r.fromPid === pid || r.toPid === pid));
  }

  // ---- camera ----
  let camS = 1, camX = 0, camY = 0, drag: { x: number; y: number } | null = null;
  // Promote the stage to its own layer only DURING interaction (smooth pan/
  // zoom), then drop will-change ~180ms after it settles so the browser
  // re-rasterises the layer at the current scale - keeping SVG/PNG logos and
  // text crisp instead of GPU-upscaling a stale 1x bitmap (the "blurry until
  // hover" artifact).
  let settleT: number | undefined;
  const apply = () => {
    stage.style.transform = `translate(${camX}px,${camY}px) scale(${camS})`;
    stage.style.willChange = "transform";
    if (settleT) clearTimeout(settleT);
    settleT = window.setTimeout(() => { stage.style.willChange = "auto"; }, 180);
  };
  function fit() {
    const items = [...stage.querySelectorAll<HTMLElement>(".region,.hive,.actor")]; if (!items.length) return;
    let a = 1e9, b = 1e9, c = -1e9, d = -1e9;
    items.forEach((el) => { const l = parseFloat(el.style.left), t = parseFloat(el.style.top), w = el.offsetWidth, h = el.offsetHeight; a = Math.min(a, l); b = Math.min(b, t); c = Math.max(c, l + w); d = Math.max(d, t + h); });
    const pad = 42, bw = c - a + pad * 2, bh = d - b + pad * 2;
    camS = Math.min(cam.clientWidth / bw, cam.clientHeight / bh, 1.05);
    camX = (cam.clientWidth - bw * camS) / 2 - (a - pad) * camS; camY = (cam.clientHeight - bh * camS) / 2 - (b - pad) * camS; apply();
  }
  const onDown = (e: MouseEvent) => { if ((e.target as HTMLElement).closest(".hive,.pop")) return; closePop(); drag = { x: e.clientX - camX, y: e.clientY - camY }; cam.classList.add("drag"); };
  const onMove = (e: MouseEvent) => { if (!drag) return; camX = e.clientX - drag.x; camY = e.clientY - drag.y; apply(); };
  const onUp = () => { drag = null; cam.classList.remove("drag"); };
  const onWheel = (e: WheelEvent) => {
    e.preventDefault();
    // Smooth zoom: scale by exp of the wheel delta (the standard approach - a
    // small per-notch factor instead of a hard 1.1x/0.9x jump), clamped so it
    // can't zoom into oblivion or out to nothing. deltaMode 1 (lines) reports
    // ~1/100th the magnitude of pixel deltas, so normalise it first.
    const px = e.deltaMode === 1 ? e.deltaY * 16 : e.deltaY;
    const r = cam.getBoundingClientRect(), mx = e.clientX - r.left, my = e.clientY - r.top;
    const ns = Math.min(4, Math.max(0.15, camS * Math.exp(-px * 0.0016)));
    const f = ns / camS;
    camX = mx - (mx - camX) * f; camY = my - (my - camY) * f; camS = ns; apply();
  };
  cam.addEventListener("mousedown", onDown); window.addEventListener("mousemove", onMove); window.addEventListener("mouseup", onUp); cam.addEventListener("wheel", onWheel, { passive: false });
  const onResize = () => fit(); window.addEventListener("resize", onResize);

  const ctrls = document.createElement("div"); ctrls.className = "ctrls";
  ctrls.innerHTML = `<button data-a="in">+</button><button data-a="out">−</button><button data-a="fit">⤢</button>`;
  ctrls.addEventListener("click", (e) => { const a = (e.target as HTMLElement).dataset.a; if (a === "in") { camS *= 1.15; apply(); } else if (a === "out") { camS *= 0.87; apply(); } else if (a === "fit") fit(); });
  container.appendChild(ctrls);

  // Edge-colour legend. Public/host access (blue) vs in-cluster traffic (grey)
  // are always present; provisioning (violet) + escape (red) only when a lab
  // session is live, so they're listed conditionally. "observes" earns a key
  // because the eye is a new glyph rather than a colour of an existing one - it
  // only appears for eBPF modules, so it is listed only when one is drawn.
  const legend = document.createElement("div"); legend.className = "legend";
  const key = (c: string, l: string) => `<span style="color:${c}"><i></i>${l}</span>`;
  const hasObserves = (data.sessions || []).some((s) => (s.cluster?.edges || []).some((e) => e.kind === "observes"));
  legend.innerHTML = (scoped ? "" : key("var(--tp-ext)", "public / host access")) + key("var(--tp-ink3)", "in-cluster")
    + ((data.sessions || []).length ? key("var(--tp-violet)", "provisioning") + key("#ff5b6b", "escape path") : "")
    + (hasObserves ? key("var(--tp-accent)", "observes (eBPF)") : "");
  container.appendChild(legend);

  fit(); requestAnimationFrame(fit); const t1 = window.setTimeout(fit, 250);

  return () => {
    window.clearTimeout(t1);
    window.removeEventListener("mousemove", onMove); window.removeEventListener("mouseup", onUp); window.removeEventListener("resize", onResize);
    container.innerHTML = ""; container.classList.remove("tp-root");
  };
}
