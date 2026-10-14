import { useRef } from "react";
import { Camera, X } from "lucide-react";
import { Avatar } from "./Avatar";
import { type User } from "../../lib/api";

// The avatar IS the upload control.
//
// It used to be an image next to an "Upload image" button next to a "Remove"
// button next to a line of text listing accepted formats and a byte limit -
// four elements to change one picture. Clicking the thing you want to change
// is the convention everywhere else (Slack, GitHub, Google), so the image
// carries the affordance: hover reveals a camera scrim, click opens the file
// picker.
//
// The accepted formats are not listed. The file picker already filters to
// them, and a user who somehow picks a TIFF gets a specific error back from
// the server - which is more useful at the moment it happens than a permanent
// line of small print is beforehand.
export function AvatarPicker({ user, version, onPick, onRemove, busy }: {
  // The same slice <Avatar> renders; the picker adds the upload affordances
  // around it rather than re-deciding image-vs-initial itself.
  user: Pick<User, "id" | "username" | "hasAvatar">;
  version?: number;
  onPick: (file: File) => void;
  onRemove?: () => void;
  busy?: boolean;
}) {
  const input = useRef<HTMLInputElement>(null);

  return (
    <div className="group relative h-20 w-20 shrink-0">
      <input
        ref={input}
        type="file"
        accept="image/png,image/jpeg,image/gif,image/webp"
        className="hidden"
        onChange={(e) => {
          const f = e.target.files?.[0];
          if (f) onPick(f);
          // Cleared so picking the same file twice still fires a change event.
          e.target.value = "";
        }}
      />

      <button
        type="button"
        onClick={() => input.current?.click()}
        disabled={busy}
        aria-label="Change your profile picture"
        className="relative block h-20 w-20 overflow-hidden rounded-full ring-1 ring-slate-700
          transition hover:ring-slate-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
      >
        <Avatar user={user} size={80} version={version} />

        {/* The scrim is the hint. Always present for a keyboard user via
            focus-visible, otherwise revealed on hover so a set avatar isn't
            permanently dimmed by its own edit affordance. */}
        <span
          className="absolute inset-0 flex items-center justify-center bg-slate-950/60 text-slate-100
            opacity-0 transition-opacity group-hover:opacity-100"
        >
          <Camera size={20} />
        </span>
      </button>

      {user.hasAvatar && onRemove && (
        <button
          type="button"
          onClick={onRemove}
          disabled={busy}
          aria-label="Remove your profile picture"
          title="Remove"
          className="absolute -right-1 -top-1 flex h-6 w-6 items-center justify-center rounded-full
            border border-slate-700 bg-slate-900 text-slate-400 opacity-0 transition
            hover:border-red-500/40 hover:text-red-400 group-hover:opacity-100
            focus:opacity-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        >
          <X size={13} />
        </button>
      )}
    </div>
  );
}
