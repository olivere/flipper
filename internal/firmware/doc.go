// Package firmware exposes TRMNL firmware support to the rest of
// Flipper. It has three responsibilities:
//
//  1. Read-only release metadata from the upstream
//     usetrmnl/trmnl-firmware GitHub repository (releases.go). Used
//     by `flipper firmware list` and `flipper firmware status`. No
//     binaries are downloaded by this path. ListReleases caches
//     responses in-process for 15 minutes to stay under the 60
//     req/hour unauthenticated rate limit.
//
//  2. A local binary Store (store.go) that imports .bin files from a
//     path or URL, records (version, model, sha256) in a manifest,
//     and serves files back by filename. Imports must declare the
//     target model; re-importing the same (version, model) with
//     different bytes is refused so the operator removes the old
//     copy first.
//
//  3. A Pending arm tracker (pending.go) that records one-shot OTA
//     dispatches per MAC. Take consumes an arm atomically under a
//     file lock, so each arm dispatches at most once and a failed
//     flash never auto-retries — the operator must re-arm.
package firmware
