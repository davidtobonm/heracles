# Configure preferences through role key-value pairs

Heracles will expose one consistent Preference interface for Planner, Issue Author, Implementer, and Reviewer. `heracles config` manages project Preferences by default, while `heracles config -g` manages user-wide Preferences; both accept concise key/value pairs such as `implementer.model "gpt-5.5"` and `reviewer.provider "opencode"`. With no key/value pairs the command shows effective Preferences, `--unset` removes selected keys, and `--path` prints the active Preferences file. Unknown keys and values incompatible with the selected provider fail immediately.

Preferences may also override the Planning Question Budget, per-Labor issue limit and concurrency, automatic merging, and Issue Workspace cleanup or preservation behavior. Repository identities and paths, verification commands, base branches, merge order, and the Issue Tracker remain shared portable Project Configuration. Launch flags, project Preferences, global Preferences, and portable Project Configuration apply in that precedence order, allowing personal choices without editing shared `heracles.yaml`.

The saved `labor.issue-limit` applies to both `heracles labor` and direct `heracles run`. A launch may override it with `--limit N`, the equivalent dot-key, or `--no-limit` to drain every eligible Ready Issue. Reaching the limit is a successful resumable pause rather than a blocked or failed Labor. Within any invocation, Heracles continues refreshing the Issue Tracker after each completed batch so issues unblocked by earlier work in the same run remain eligible before the limit or backlog boundary is reached.

Launch commands support both explicit dashed flags such as `--implementer-model gpt-5.5` and dot-key pairs such as `implementer.model gpt-5.5` as first-class syntax. Both forms have identical meaning and validation; supplying conflicting values for the same Preference is an error rather than an implicit precedence choice.

`heracles labor` accepts the problem as either concise positional text or through `--problem`; conflicting values are rejected. It generates a readable Labor ID unless the user supplies `--id`, preserving concise interactive use and explicit scripting.

Direct validated `heracles config` key/value changes write immediately and report the changed keys. Removing multiple keys asks for confirmation unless `--yes` is supplied. Interactive `heracles init` shows one final summary or diff and asks for confirmation before atomically writing Project Configuration and Preferences. Validation failure produces no partial writes.

List Preferences use repeated dot keys, such as repeated `implementer.extra-arg` or `implementer.env-allow` pairs. Mentioning a list key replaces its stored list by default; `--append` explicitly extends the existing list.
