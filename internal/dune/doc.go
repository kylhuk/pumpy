// Package dune provides a small client and normalization helpers for Dune SVM
// transaction payloads.
//
// Normalization handles a few encoding variants seen in Dune responses:
//   - block_time values in second/millisecond/microsecond/nanosecond precision
//   - accountKeys encoded as []string or []{"pubkey":...}
//   - instruction accounts encoded as []int indexes or []string addresses
package dune
