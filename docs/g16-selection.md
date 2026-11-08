# G16: representative bounded file selection

The selector ranks paths for inspection; paths never prove a project role.
No repository identity is part of its rules. Default limits remain 16 files,
6 generic source files and 4 reserved entry sources. MaxPerBase remains 3 for
repeated docs/source/config basenames. Component-defining manifests use their
component directory as part of that key, so independent packages are governed
by the total file cap rather than hiding one another under a global filename.

Product-scope candidates precede supporting/test paths. Primary entry sources
are reserved before ordinary filling; supporting entries can be selected later
if space remains. Non-test Go module-root and cmd package files receive higher
priority to preserve package-clause evidence needed by classification. The
nearest supported component boundary determines the Go module context.

Declared nested component names and the first applicable cmd/<binary> identity
are opaque names, including tool/build/docs. Supporting ancestors and fixtures
inside a command remain supporting even when they carry manifests or nested
cmd directories. Command scope casing agrees with the classifier. Symlinks are
still excluded; all candidates use the same total/source/basename accounting.

The output order uses scope, score, depth and path; Score alone is not a global
sort key. Selection is deterministic for a given tree, independent of API order.
Tiny budgets can still omit evidence, and arbitrarily many components cannot
be exhaustively inspected. These cases must retain explained unknown results.

Historical capture selections and bytes remain unchanged. Current-selector
tests consume only captured bytes actually selected now; missing new inputs
are explicitly unavailable. Eight immutable capture hashes guard provenance.
The six-repository offline corpus now yields five correctly resolved roles and
one justified abstention, versus three resolved roles under the historical
selection. This is a small corpus result, not general classification accuracy.

BuilderVersion 5 invalidates previews acquired with the older selection, while
current-version cache reuse and fresh visibility checks remain in place.
The [G19 limits](analysis-limits.md) continue to bound physical requests, bytes
and deadlines. G15's Python framework-role boundary, unread Go root alternatives
and classifier directory-scope policies remain separate work.

Regression evidence and review findings are retained in the shared
[execution record](../../docs/2026-11-06-g16-selection/README.md).
