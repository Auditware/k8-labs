import type { ReactNode } from "react";
import { renderDiagram, type Diagram } from "./course/diagram";

// The one-page guide for the (course-less) research labs. It is a course concept
// STEP in everything but the data: the same eyebrow, the same DiagramStage card,
// the same text-26 title and cd-prose text-18 body the CoursePlayer renders (see
// CoursePlayer.tsx concept branch) - so it is visually identical to a guided
// lab's screen, only the content differs. Do not restyle it away from the course.

type Props = {
  description?: string;
  // hardened = the -hard lab (mitigations on); tweaks one caption.
  hardened?: boolean;
  // picker is the optional driver-selection panel (DriverPicker), rendered at
  // the foot of the scrolling body. LabPage owns its state; ResearchGuide just
  // gives it a home so the layout stays one column.
  picker?: ReactNode;
};

// The exploit flow inside the throwaway microVM, in the shared 640x300 diagram
// space (content x 20-620, caption band y 240-270). Same shapes/tones/animations
// as the guided courses.
function flowDiagram(hardened?: boolean): Diagram {
  return {
    caption: "eBPF here is instrumentation, not a challenge - live low-level tracing of what the kernel does",
    shapes: [
      {
        type: "frame",
        x: 20,
        y: 74,
        w: 600,
        h: 158,
        label: "throwaway Firecracker microVM · a KASAN research kernel",
      },
      { type: "box", x: 34, y: 100, w: 132, h: 50, label: "load driver", sub: "insmod the module", delay: 0 },
      { type: "box", x: 182, y: 100, w: 132, h: 50, label: "fuzz ioctls", sub: "trip the UAF", delay: 0.12 },
      { type: "box", x: 330, y: 100, w: 132, h: 50, label: "KASAN oracle", sub: "names the object", tone: "accent", delay: 0.24 },
      {
        type: "box",
        x: 478,
        y: 100,
        w: 132,
        h: 50,
        label: "uid 0 · root",
        sub: hardened ? "cross-cache → cred" : "groom → cred",
        tone: "danger",
        delay: 0.36,
      },
      { type: "wire", from: { x: 166, y: 125 }, to: { x: 182, y: 125 }, delay: 0.18 },
      { type: "wire", from: { x: 314, y: 125 }, to: { x: 330, y: 125 }, tone: "accent", delay: 0.3 },
      { type: "wire", from: { x: 462, y: 125 }, to: { x: 478, y: 125 }, tone: "danger", delay: 0.42 },
      { type: "box", x: 206, y: 178, w: 228, h: 38, label: "eBPF · Tetragon", sub: "live behavioral analysis", tone: "accent", delay: 0.55 },
      { type: "wire", from: { x: 500, y: 150 }, to: { x: 408, y: 178 }, tone: "accent", delay: 0.62 },
    ],
  };
}

export default function ResearchGuide({ description, hardened, picker }: Props) {
  const html = renderDiagram(flowDiagram(hardened));
  return (
    <div className="flex h-full min-h-0 flex-col">
      {/* eyebrow - identical to the course player's. The blue tag carries the
          variant (the course's step-kind slot), so the eyebrow names both facts -
          variant + family - and nothing downstream has to repeat them. */}
      <div className="flex shrink-0 items-center gap-2 px-5 pt-4 font-mono text-11 uppercase tracking-wider text-slate-500">
        <span className="text-blue-400">{hardened ? "HARDENED" : "DEV"}</span>
        <span>Kernel Driver Research</span>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-5 pb-4 pt-3">
        {/* diagram - the DiagramStage wrapper the course player uses */}
        <div className="relative mb-4 rounded-lg border border-slate-800 bg-slate-900/60 p-2">
          <div className="cd-stage" dangerouslySetInnerHTML={{ __html: html }} />
        </div>

        {/* title + body - the concept step's classes verbatim. Inline terms use the
            course term chip (.k / .k.cmd, see index.css) so paths, commands and kernel
            symbols read exactly like a guided lab instead of bare monospace. */}
        <h3 className="text-26 font-semibold leading-snug text-slate-100">
          You attack the kernel, not the cluster.
        </h3>
        <div className="cd-prose mt-3 text-18 leading-relaxed text-slate-300">
          {description && <p>{description}</p>}
          <ul className="my-3 list-disc space-y-2 pl-5 marker:text-slate-600">
            <li>
              <b>Build and load</b> - a loadable-module build tree, so <span className="k">make</span>{" "}
              in <span className="k">/app/driver</span> works in-session.
            </li>
            <li>
              <b>A starting point</b> - a sample vulnerable driver and a starter ioctl fuzzer
              under <span className="k">/app</span>.
            </li>
            {hardened ? (
              <li>
                <b>Production hardening on</b> - <span className="k">KASLR</span>, randomized
                kmalloc caches, a hardened freelist and{" "}
                <span className="k">STATIC_USERMODEHELPER</span>, so the easy groom stops working
                and you find out whether the technique really survives.
              </li>
            ) : (
              <li>
                <b>Mitigations off</b> - a clean bench for bringing a technique up before
                anything fights back.
              </li>
            )}
            <li>
              <b>Live view</b> - the eBPF pane instruments the kernel live, so you can trace and
              analyze what your exploit actually does.
            </li>
          </ul>
          <p>
            No guided course and no flag - it&#39;s a discovery sandbox. Bring your own module, or
            start from the sample driver, and watch the Defender tab for what the kernel sees.
          </p>
        </div>

        {/* Second panel - a concise walkthrough of the sample-driver path, so a
            researcher new to the bench has a spine to follow without any on-disk
            files. General purpose: swap in your own driver at step 1. */}
        <div className="mt-5 rounded-lg border border-slate-800 bg-slate-900/60 p-4">
          <div className="font-mono text-11 uppercase tracking-wider text-blue-400">
            Working the lab
          </div>
          <ol className="cd-prose mt-2 list-decimal space-y-1.5 pl-5 text-15 leading-relaxed text-slate-300 marker:text-slate-600">
            <li>
              <b>Load</b> - <span className="k cmd">sh /app/driver/load.sh</span> builds and{" "}
              <span className="k">insmod</span>s the sample driver at{" "}
              <span className="k">/dev/kuaf</span> (or <span className="k cmd">make</span> and{" "}
              <span className="k">insmod</span> your own).
            </li>
            <li>
              <b>Fuzz</b> - <span className="k cmd">cd /app/fuzz &amp;&amp; ./fuzz uaf</span> trips the
              use-after-free; the KASAN report in <span className="k cmd">dmesg</span> names the
              object and its <span className="k">kmalloc-64</span> cache.
            </li>
            <li>
              <b>Reclaim</b> - free the object, then spray a controllable one (
              <span className="k">msg_msg</span>, <span className="k">setxattr</span>,{" "}
              <span className="k">pipe_buffer</span>) into the freed slot so you own its bytes.
            </li>
            <li>
              <b>Escalate</b> - finish data-only: overwrite a <span className="k">cred</span> to{" "}
              <span className="k">uid 0</span>
              {hardened ? " - cross-cache into cred_jar and defeat KASLR first" : " by grooming the reclaim directly"}.
            </li>
            <li>
              <b>Instrument</b> - confirm root (<span className="k cmd">id</span> &rarr; 0) and use
              the eBPF pane to trace the exploit&#39;s real footprint in the kernel.
            </li>
          </ol>
        </div>

        {picker}
      </div>
    </div>
  );
}
