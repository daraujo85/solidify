# Validation Plan - SAI-127 (Provider Resilience & Model Failover)

> **Status:** `SAI-127: IMPLEMENTED_PENDING_VALIDATION`
> **Blocked by:** Go toolchain unavailable, executable Git test repository unavailable.

Execute this 18-point plan sequentially when the environment becomes available.

## Section A: Static & Contract Verification
1. [ ] Compile check: `go build ./...` must succeed.
2. [ ] Unit tests execution: `go test ./internal/...` must pass (especially `ai`, `errs`, `peer`, `arbiter`).
3. [ ] Verify `internal/errs/public.go` explicitly remaps `CodeIncomplete` to exit code `2` without stripping internal metadata.

## Section B: Resilience Categories (FakeProvider tests)
4. [ ] Run `FakeProvider` test injecting `quota exceeded` and ensure it maps to `QUOTA_EXHAUSTED`.
5. [ ] Run `FakeProvider` test injecting `prepayment credits` and ensure it maps to `CREDITS_DEPLETED`.
6. [ ] Run `FakeProvider` test injecting `invalid_api_key` and ensure it maps to `INVALID_CREDENTIALS`.
7. [ ] Run `FakeProvider` test injecting `rate_limit_exceeded` without quota strings and ensure it maps to `RATE_LIMITED`.

## Section C: Failover Flow (Arbiter & Peer)
8. [ ] Execute `PeerExecutor` with `MaxRetries=1`, force a `QUOTA_EXHAUSTED` on attempt 1 and success on attempt 2.
9. [ ] Validate that `ExecutorResult.Attempts` length is 2.
10. [ ] Validate `Attempts[0].Result == "error"` and `Attempts[0].ErrorCategory == "QUOTA_EXHAUSTED"`.
11. [ ] Validate `Attempts[1].Result == "success"`.
12. [ ] Perform the same attempt sequence on `ArbiterExecutor`.

## Section D: Partial Report & Defer (run.go)
13. [ ] Trigger `solidify run` configured with an always-failing `FakeProvider` (e.g., `MaxRetries=0`, failing immediately).
14. [ ] Observe process exit code is `2`.
15. [ ] Assert `.solidify/out/release-report.json` is generated despite the failure.
16. [ ] Assert `status == "INCOMPLETE"` and `failed_stage == "AI_ORCHESTRATION"` in the report JSON.
17. [ ] Assert `reason_code == 21` (or the equivalent exit code associated with AI failure) and `failure.category == "QUOTA_EXHAUSTED"`.
18. [ ] Assert `independence_gate` uses `ExecutedModel` not `RequestedModel` to calculate `distinct_models`.
