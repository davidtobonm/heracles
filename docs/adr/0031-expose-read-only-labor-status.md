# Expose read-only Labor status

Heracles exposes `heracles status [labor-id]`, defaulting to the current or most
recent Labor. It reports the stage, Defined Backlog progress, active issues,
blockers, pull requests, limits, and appropriate resume guidance.

`heracles status --json` returns machine-readable output. Status reads local
Labor state and current GitHub delivery facts without mutating either.
