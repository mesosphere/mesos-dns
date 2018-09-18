# How to Release

We release Mesos-DNS once every 3-4 months or so. We try to release several release candidates prior to a release.

## Releasing an RC

1. Tag a commit `git tag v0.5.3-rc1` (example).
2. `git push --tags`.
3. Rebuild it in CircleCI without cache.
4. Upload artifacts from CircleCI to Github.

## Releasing

1. Cut a branch named like `release-v0.5.3`.
2. Populate `CHANGELOG` as to differences from the last release.
3. Commit it with a message like "Releasing v0.5.3".
4. Tag this specific commit as `v0.5.3`.
5. `git push --tags`.
6. Upload artifacts from CircleCI to Github.
