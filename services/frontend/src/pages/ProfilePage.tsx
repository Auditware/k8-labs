import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, Globe } from "lucide-react";

import { getPublicProfile, ProfileNotFound, publicAvatarUrl, type PublicProfile } from "../lib/api";
import { XIcon, LinkedInIcon } from "../components/BrandIcons";
import { formatSolve, joinedAgo, topPercentile } from "../lib/format";
import { Avatar } from "../components/ui/Avatar";
import { Medal, type Tone } from "../components/ui/Medal";
import { ActivityHeatmap } from "../components/ActivityHeatmap";
import { HeroEscapeBackground } from "../components/HeroEscapeBackground";
import { PublicNav } from "../layout/PublicNav";
import { cn } from "../lib/cn";
import { SITE_URL, useSeo } from "../lib/seo";

// The public operator profile at /u/:username. Everything on it is real and
// matches the leaderboard by construction (same ranked board backs both): the
// standing, the headline stats, and the activity calendar - which is genuine
// finished-session history, so an idle account shows an empty grid rather than a
// manufactured one.

const MEDALS: { ring: string; tone: Tone }[] = [
  { ring: "ring-[#e6c36b]", tone: "gold" },
  { ring: "ring-[#bfd0e0]", tone: "silver" },
  { ring: "ring-[#ce8a55]", tone: "bronze" },
];

function Stat({ k, v }: { k: string; v: string }) {
  return (
    <div className="text-center">
      <div className="font-mono text-22 font-bold tabular-nums text-slate-100">{v}</div>
      <div className="mt-1 font-mono text-10 uppercase tracking-wider text-slate-500">{k}</div>
    </div>
  );
}

function Shell({ children }: { children: React.ReactNode }) {
  return (
    <div className="relative min-h-screen overflow-x-hidden bg-slate-950 text-slate-100">
      <HeroEscapeBackground className="pointer-events-none fixed inset-0 h-full w-full" />
      <PublicNav />
      <main className="relative z-10 mx-auto max-w-3xl px-4 pb-20 pt-40 sm:px-6">
        <Link
          to="/leaderboard"
          className="mb-6 inline-flex items-center gap-1.5 font-mono text-11 uppercase tracking-wider text-slate-500 transition-colors hover:text-slate-300"
        >
          <ArrowLeft size={13} aria-hidden /> leaderboard
        </Link>
        {children}
      </main>
    </div>
  );
}

function Body({ p }: { p: PublicProfile }) {
  const medal = p.rank <= 3 && p.labsCompleted > 0 ? MEDALS[p.rank - 1] : null;
  const pct = topPercentile(p.rank, p.totalOperators);
  const user = { id: "", username: p.username, hasAvatar: p.hasAvatar };

  return (
    <div className="rounded-2xl border border-hairline bg-surface p-6 shadow-[0_40px_90px_-50px_rgba(0,0,0,0.9)] sm:p-8">
      <div className="flex flex-wrap items-start gap-x-5 gap-y-4">
        <span className="relative inline-flex shrink-0">
          <Avatar
            user={user}
            size={64}
            src={p.hasAvatar ? publicAvatarUrl(p.username) : undefined}
            className={cn("ring-1", medal ? `ring-2 ${medal.ring}` : "ring-hairline")}
          />
          {medal && (
            <span className="absolute -bottom-2 -right-1.5 leading-none">
              <Medal tone={medal.tone} size={28} />
            </span>
          )}
        </span>

        <div className="min-w-[12rem] flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-bold text-slate-100">{p.username}</h1>
            <span className="rounded-full border border-accent/35 bg-accent/10 px-2 py-0.5 font-mono text-10 text-accent">
              RANK #{p.rank}
            </span>
          </div>
          <div className="mt-1 font-mono text-xs text-slate-500">
            {joinedAgo(p.joinedAt)}
          </div>
          {p.bio && <p className="mt-3 max-w-prose text-sm text-slate-300">{p.bio}</p>}
          {p.website && (
            <a
              href={p.website}
              target="_blank"
              rel="noreferrer noopener nofollow ugc"
              className="mt-2 inline-flex items-center gap-1.5 text-13 text-accent transition-colors hover:text-[#7fb4f5]"
            >
              <Globe size={13} aria-hidden />
              {p.website.replace(/^https?:\/\//, "")}
            </a>
          )}
          {(p.x || p.linkedin) && (
            <div className="mt-3 flex items-center gap-3">
              {p.x && (
                <a
                  href={p.x}
                  target="_blank"
                  rel="noreferrer noopener nofollow ugc"
                  aria-label="X profile"
                  className="text-slate-400 transition-colors hover:text-slate-100"
                >
                  <XIcon className="h-4 w-4" />
                </a>
              )}
              {p.linkedin && (
                <a
                  href={p.linkedin}
                  target="_blank"
                  rel="noreferrer noopener nofollow ugc"
                  aria-label="LinkedIn profile"
                  className="text-slate-400 transition-colors hover:text-[#0a66c2]"
                >
                  <LinkedInIcon className="h-4 w-4" />
                </a>
              )}
            </div>
          )}
        </div>
      </div>

      <div className="mt-7 grid grid-cols-2 gap-y-5 border-y border-hairline py-5 sm:flex sm:justify-between sm:gap-6">
        <Stat k="labs cleared" v={String(p.labsCompleted)} />
        <Stat k="best time to root" v={p.fastestSolveSeconds != null ? formatSolve(p.fastestSolveSeconds) : "-"} />
        <Stat k="day streak" v={String(p.longestStreakDays)} />
        {pct && <Stat k="top percentile" v={pct} />}
      </div>

      <div className="mt-6">
        <ActivityHeatmap activity={p.activity} days={p.activityDays} />
      </div>
    </div>
  );
}

export default function ProfilePage() {
  const { username = "" } = useParams();
  const query = useQuery({
    queryKey: ["profile", username],
    queryFn: () => getPublicProfile(username),
    retry: (count, err) => !(err instanceof ProfileNotFound) && count < 2,
    refetchOnWindowFocus: false,
  });

  const p = query.data;
  const notFound = query.error instanceof ProfileNotFound;
  useSeo({
    title: p ? `${p.username} - Profile · K8 Labs` : "Profile · K8 Labs",
    description: p
      ? `${p.username} is ranked #${p.rank} on K8 Labs with ${p.labsCompleted} lab${p.labsCompleted === 1 ? "" : "s"} cleared.`
      : "A track record on the K8 Labs Kubernetes attack & defense leaderboard.",
    path: `/u/${username}`,
    type: "profile",
    // A username that isn't on the board is a 404-equivalent - don't index it.
    noindex: notFound,
    jsonLd: p
      ? {
          "@context": "https://schema.org",
          "@type": "ProfilePage",
          mainEntity: {
            "@type": "Person",
            name: p.username,
            url: `${SITE_URL}/u/${p.username}`,
          },
        }
      : undefined,
  });

  if (query.isPending) {
    return (
      <Shell>
        <div className="rounded-2xl border border-hairline bg-surface p-8" aria-hidden>
          <div className="flex items-center gap-5">
            <div className="h-16 w-16 animate-pulse rounded-full bg-slate-800" />
            <div className="flex-1 space-y-3">
              <div className="h-5 w-40 animate-pulse rounded bg-slate-800" />
              <div className="h-3 w-56 animate-pulse rounded bg-slate-800" />
            </div>
          </div>
          <div className="mt-8 h-24 animate-pulse rounded bg-slate-800/60" />
        </div>
      </Shell>
    );
  }

  if (query.isError) {
    return (
      <Shell>
        <div className="rounded-2xl border border-hairline bg-surface px-6 py-16 text-center">
          <h1 className="text-17 font-semibold text-slate-100">
            {notFound ? "No such user" : "Couldn't load this profile"}
          </h1>
          <p className="mx-auto mt-1 max-w-sm text-sm text-slate-400">
            {notFound ? (
              <>
                There's no user named <span className="font-mono text-slate-300">{username}</span> on the board.
              </>
            ) : (
              "Try again shortly."
            )}
          </p>
        </div>
      </Shell>
    );
  }

  return (
    <Shell>
      <Body p={query.data} />
    </Shell>
  );
}
