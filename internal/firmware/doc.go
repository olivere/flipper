// Package firmware surfaces TRMNL firmware release metadata from the
// upstream usetrmnl/trmnl-firmware GitHub repository. It is strictly
// read-only — no binaries are downloaded and no device state is changed.
//
// The public GitHub API is unauthenticated and capped at 60 requests/hour
// per IP, so ListReleases caches results in-process for 15 minutes.
package firmware
