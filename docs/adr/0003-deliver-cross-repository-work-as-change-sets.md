# Deliver cross-repository work as Change Sets

Heracles will deliver an issue as a Change Set containing one linked pull request per touched target repository. Pull requests must pass configured local verification and CI before merging in a configured order; because cross-repository delivery cannot be atomic, a merge failure leaves remaining pull requests open and marks the issue blocked. Automatic merging is opt-in.
