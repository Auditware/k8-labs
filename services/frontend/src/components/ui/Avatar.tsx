import { useState } from "react";
import { avatarUrl, type User } from "../../lib/api";

// Matches the backend's MinAvatarPx. An image below it (the case that prompted
// this: a 1x1 transparent PNG) would render as an empty circle, so it gets the
// initial instead. Uploads under the floor are now rejected server-side; this
// covers rows stored before that check.
const MIN_AVATAR_PX = 16;

// The one place a user's profile circle is drawn: the uploaded image when there
// is one, an initial otherwise. Every profile circle in the app should be this
// component so a new avatar shows up everywhere at once, and so the fallback
// (first letter, on a blue chip) is defined in a single spot rather than
// re-derived at each call site.
//
// It needs only id/username/hasAvatar, so it takes a slice of User rather than
// the whole thing - that lets mock/list rows that aren't full accounts render
// too. `version` is a cache-buster: bump it after an upload/remove so the
// browser drops the avatar endpoint's cached bytes (see SettingsPage).
export function Avatar({
  user,
  size = 36,
  version,
  src,
  className = "",
}: {
  user: Pick<User, "id" | "username" | "hasAvatar">;
  size?: number;
  version?: number;
  // Override the image URL. Public surfaces (profiles, landing) pass a
  // handle-based publicAvatarUrl so no internal account id rides a public page;
  // when unset the authenticated avatarUrl(id) is used, as before.
  src?: string;
  className?: string;
}) {
  const url = src ?? avatarUrl(user.id, version);
  // Keyed on the URL so a new upload (version bump) gets a fresh chance.
  const [badUrl, setBadUrl] = useState<string | null>(null);
  const showImage = user.hasAvatar && badUrl !== url;

  return (
    <span
      style={{ width: size, height: size }}
      className={`inline-flex shrink-0 overflow-hidden rounded-full ${className}`}
    >
      {showImage ? (
        <img
          src={url}
          alt=""
          className="h-full w-full object-cover"
          onError={() => setBadUrl(url)}
          onLoad={(e) => {
            const img = e.currentTarget;
            if (img.naturalWidth < MIN_AVATAR_PX || img.naturalHeight < MIN_AVATAR_PX) setBadUrl(url);
          }}
        />
      ) : (
        <span
          style={{ fontSize: size * 0.42 }}
          className="flex h-full w-full items-center justify-center bg-blue-500/20 font-semibold text-blue-300"
        >
          {user.username.slice(0, 1).toUpperCase()}
        </span>
      )}
    </span>
  );
}
