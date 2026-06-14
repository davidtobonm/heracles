# Allow only one active Labor per project

Heracles allows only one active Labor per configured project at a time. Starting
or resuming another Labor while the project lock is held fails clearly and
identifies the active Labor.

The single active Labor may still execute independent implementation issues
concurrently according to `labor.concurrency`, which defaults to `1`. Higher
concurrency uses isolated workspaces. Heracles prevents duplicate work on the
same semantic issue and never runs dependency-linked or repository-conflicting
issues simultaneously.

The Issue Author declares each implementation issue's target repositories and
optional conflict keys. Heracles serializes issues that share either. Missing
or uncertain targets are serialized conservatively. If runtime overlap is
discovered after execution starts, Heracles blocks one issue temporarily and
safely retries it after the conflicting issue completes.

Heracles automatically removes a lock only when local process state proves it
is stale. Users cannot force-unlock a project, and GitHub state is not consulted
to reconstruct or override the lock.
