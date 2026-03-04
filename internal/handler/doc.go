// Package handler implements the HTTP API that TRMNL devices call.
//
// Endpoints:
//
//	GET  /api/setup          — device registration (returns API key)
//	GET  /api/display         — returns current screen image metadata
//	POST /api/log             — accepts device log reports
//	GET  /images/{filename}   — serves processed image files
//
// Handler holds shared dependencies (config, device registry, screen
// registry, image pipeline, cache, logger) injected by the server
// package at startup.
package handler
