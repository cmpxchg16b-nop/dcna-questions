// resolveAssetSrc normalizes an asset URL coming from an exam document for
// use as an <img> src. The server emits root-absolute URLs ("/assets/..." for
// static exams, "/api/dyn-assets/..." for VFS-backed ones) but exam documents
// may also carry bare relative references ("assets/..."); only the latter
// need the leading slash added. Blindly prepending "/" to an absolute URL
// would yield a protocol-relative "//host/..." URL pointing at a nonexistent
// host.
export function resolveAssetSrc(src: string): string {
  return src.startsWith("/") ? src : `/${src}`;
}
