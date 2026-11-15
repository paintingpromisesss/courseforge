# Build artifact retention

`Release` uploads one Actions artifact for each of six platforms. Tag runs
download them to publish GitHub Release assets; branch dispatches do not publish
a release. CI does not upload artifacts. Published releases also feed the app's
updater and are never managed by artifact maintenance.

`Artifact maintenance` runs after a **successful completed** `Release`. It keeps
the two newest successful runs with all six platforms, ordered by run number,
without mixing files from different runs. A failed, cancelled, partial or running
build cannot replace either set. Unknown artifact names and other workflows are
outside its deletion scope.

Before deleting anything, maintenance copies both complete sets to dedicated
**draft releases**, named `courseforge-build-archive-<run-id>-<attempt>`. Each draft
contains the six original artifact ZIPs. Its JSON body records the source run,
commit and original artifact metadata. ZIP contents, size and SHA-256 are checked
before upload; all six uploaded asset sizes and SHA-256 digests must match before
deletion is allowed. Interrupted uploads retry in a new draft with a `-retry-`
suffix, without deleting or overwriting existing assets.

Only then are older completed Release artifacts removed. The two protected runs'
Actions artifacts remain until GitHub's normal expiration. Archive drafts stay
indefinitely, including older copies; release storage is separate from Actions
artifact storage. No release, tag or existing release asset is deleted or edited.
If an archive cannot be verified, maintenance fails before
deleting artifacts.

Actions artifacts have a finite retention period (currently the repository's
90-day default). Draft release assets have no Actions retention deadline. If no
builds run for months, the Actions copies expire, while both complete archive
drafts and all published releases remain. A later build can preserve its predecessor even
if the original Actions artifacts and workflow history have expired. Drafts are
visible to repository collaborators with push access, and are excluded from
`releases/latest`, so the updater continues to use published releases. Keep these
service drafts unpublished, particularly while maintenance uploads their assets.

The separate workflow waits for completion, serializes maintenance jobs without
cancelling an active cleanup, and checks out the default branch rather than the
triggering run's code. Only its maintenance job receives `actions: write` and
`contents: write`; CI and Release permissions are unchanged.

## Manual use

With `gh` authenticated and available in `PATH`:

```sh
python3 .github/scripts/maintain_artifacts.py --repo paintingpromisesss/courseforge --report artifact-plan.json
python3 .github/scripts/maintain_artifacts.py --repo paintingpromisesss/courseforge --mode archive-only --report artifact-plan.json
# Only after reviewing the dry-run and explicitly approving deletion:
python3 .github/scripts/maintain_artifacts.py --repo paintingpromisesss/courseforge --mode apply --report artifact-plan.json
```

The default is read-only. The JSON report contains every artifact's ID, creation
and expiration dates, size, associated run, both retained sets, and exact deletion
candidates. `archive-only` makes durable copies without deleting anything.
The workflow's manual dispatch defaults to `dry-run` too.

Download an archive as an authenticated collaborator:

```sh
gh release download courseforge-build-archive-RUN_ID-ATTEMPT --repo paintingpromisesss/courseforge --dir build-backup
```

Run the safety checks locally with:

```sh
python3 -m unittest discover -s .github/scripts -p 'test_*.py'
```
