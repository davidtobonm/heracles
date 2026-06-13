# Isolate issues with per-repository worktrees

Each issue will run in an isolated Issue Workspace containing a temporary Git worktree for every target repository. Heracles will not switch branches or require a clean working tree in the user's original repository locations, allowing a Labor to operate across arbitrary repository layouts without disrupting concurrent human work.
