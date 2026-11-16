# Artifact cleanup

After a successful `Release`, maintenance retains the two newest complete,
successful six-platform sets in Actions and deletes only older completed
Release artifacts. Failed, partial, expired or running builds do not replace a
complete set. With fewer than two complete sets, nothing is deleted. Unknown
artifact names and other workflows are untouched.

Published GitHub Releases provide durable binary downloads. Cleanup never calls
the Releases API and never creates drafts, copies binaries, edits releases or
deletes release assets. Existing release publishing and CI are unchanged.

Actions artifacts still expire under GitHub's retention policy (currently 90
days). Published release assets remain available after that. **Unpublished
manual builds have no indefinite storage guarantee**; cleanup does not create
another storage system for them.

Manual runs default to `dry-run`; `apply` performs deletion. The JSON report
lists retained runs and exact candidate IDs, sizes and source metadata:

```sh
python3 .github/scripts/maintain_artifacts.py --repo paintingpromisesss/courseforge --report artifact-plan.json
python3 .github/scripts/maintain_artifacts.py --repo paintingpromisesss/courseforge --mode apply --report artifact-plan.json
python3 -m unittest discover -s .github/scripts -p 'test_*.py'
```
