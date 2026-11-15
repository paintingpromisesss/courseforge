"""Keep two complete Release builds, backed by unpublished draft release assets."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time
import zipfile


TARGETS = (
    'courseforge-linux-amd64', 'courseforge-linux-arm64',
    'courseforge-macos-amd64', 'courseforge-macos-arm64',
    'courseforge-windows-amd64.exe', 'courseforge-windows-arm64.exe',
)
ARTIFACT_NAMES = {*TARGETS, 'courseforge-windows-amd64'}
MARKER = 'courseforge-artifact-archive-v1'
PREFIX = 'courseforge-build-archive-'


def gh(*args, output=None, payload=None):
    result = subprocess.run(
        ['gh', *args], input=json.dumps(payload).encode() if payload else None,
        stdout=output if output else subprocess.PIPE, stderr=subprocess.PIPE, timeout=300,
    )
    if result.returncode:
        raise RuntimeError(result.stderr.decode('utf-8', errors='replace'))
    return result.stdout


def api(repo, endpoint, method='GET', payload=None):
    args = ['api', f'repos/{repo}/{endpoint}', '--method', method]
    if payload is not None:
        args += ['--input', '-']
    result = gh(*args, payload=payload)
    return json.loads(result) if result else None


def pages(repo, endpoint, key=None):
    data = json.loads(gh('api', f'repos/{repo}/{endpoint}?per_page=100',
                         '--paginate', '--slurp'))
    return [item for page in data for item in (page[key] if key else page)]


def inventory(repo):
    return {
        'runs': pages(repo, 'actions/workflows/release.yml/runs', 'workflow_runs'),
        'artifacts': pages(repo, 'actions/artifacts', 'artifacts'),
        'releases': pages(repo, 'releases'),
    }


def tag(build):
    run = build['run']
    return f"{PREFIX}{run['id']}-{run['run_attempt']}"


def digest_valid(value):
    return isinstance(value, str) and re.fullmatch(r'sha256:[0-9a-f]{64}', value)


def complete_artifacts(artifacts):
    return (len(artifacts) == len(TARGETS)
            and {a['name'] for a in artifacts} == set(TARGETS)
            and all(not a['expired'] and a['size_in_bytes'] > 0
                    and digest_valid(a.get('digest')) for a in artifacts))


def manifest(repo, run, artifacts):
    return {
        'format': MARKER, 'repository': repo,
        'run': {k: run[k] for k in (
            'id', 'run_number', 'run_attempt', 'head_sha', 'head_branch',
            'created_at', 'status', 'conclusion',
        )},
        'artifacts': sorted(artifacts, key=lambda a: a['name']),
    }


def archive_manifest(repo, release):
    try:
        build = json.loads(release['body'] or '')
        run = build['run']
        if (build['format'] != MARKER or build['repository'] != repo
                or not re.fullmatch(re.escape(tag(build)) + r'(?:-retry-\d+)?', release['tag_name'])
                or run['status'] != 'completed' or run['conclusion'] != 'success'
                or not complete_artifacts(build['artifacts'])):
            return None
        return build
    except (ValueError, KeyError, TypeError):
        return None


def archive_complete(build, release):
    assets = release['assets']
    return all(any(
        asset['name'] == artifact['name'] + '.zip'
        and asset['state'] == 'uploaded'
        and asset['size'] == artifact['size_in_bytes']
        and asset.get('digest') == artifact['digest']
        for asset in assets
    ) for artifact in build['artifacts'])


def plan(repo, state):
    runs = {r['id']: r for r in state['runs']}
    archives = {}
    for release in state['releases']:
        build = archive_manifest(repo, release)
        if build and archive_complete(build, release):
            archives[tag(build)] = (build, release)
    builds = {}
    for build, _ in archives.values():
        builds[tag(build)] = build
    for run in runs.values():
        artifacts = [a for a in state['artifacts']
                     if (a.get('workflow_run') or {}).get('id') == run['id']
                     and a['name'] in TARGETS]
        if (run['status'] == 'completed' and run['conclusion'] == 'success'
                and complete_artifacts(artifacts)):
            build = manifest(repo, run, artifacts)
            builds[tag(build)] = build
    keep = []
    for build in sorted(builds.values(), key=lambda b: (
            b['run']['run_number'], b['run']['run_attempt']), reverse=True):
        if build['run']['id'] not in {b['run']['id'] for b in keep}:
            keep.append(build)
        if len(keep) == 2:
            break
    protected = {b['run']['id'] for b in keep}
    delete = []
    if len(keep) == 2:
        for artifact in state['artifacts']:
            run = runs.get((artifact.get('workflow_run') or {}).get('id'))
            if (artifact['name'] in ARTIFACT_NAMES and run
                    and run['status'] == 'completed' and run['id'] not in protected
                    and run['run_number'] < keep[0]['run']['run_number']):
                delete.append(artifact)
    return {
        'repository': repo, 'keep': keep,
        'archive_required': [b for b in keep if tag(b) not in archives],
        'delete_artifacts': delete,
        'reclaim_bytes': sum(a['size_in_bytes'] for a in delete),
        'inventory': state,
    }


def ensure_archive(repo, build, releases):
    for release in releases:
        if archive_manifest(repo, release) == build and archive_complete(build, release):
            return
    archive_tag = tag(build)
    if any(r['tag_name'] == archive_tag for r in releases):
        # An interrupted upload can leave a starter asset; leave it and retry in a new draft.
        archive_tag += f'-retry-{time.time_ns()}'
    print(f"Backing up run #{build['run']['run_number']} to {archive_tag}", flush=True)
    with tempfile.TemporaryDirectory() as directory:
        for artifact in build['artifacts']:
            filename = artifact['name'] + '.zip'
            path = Path(directory) / filename
            with path.open('wb') as output:
                gh('api', f"repos/{repo}/actions/artifacts/{artifact['id']}/zip",
                   output=output)
            actual = 'sha256:' + hashlib.sha256(path.read_bytes()).hexdigest()
            if actual != artifact['digest'] or path.stat().st_size != artifact['size_in_bytes']:
                raise RuntimeError(f'Artifact checksum/size mismatch: {artifact["id"]}')
            with zipfile.ZipFile(path) as archive:
                if archive.namelist() != [artifact['name']] or archive.testzip():
                    raise RuntimeError(f'Unexpected artifact contents: {artifact["id"]}')
        created = api(repo, 'releases', 'POST', {
            'tag_name': archive_tag, 'target_commitish': build['run']['head_sha'],
            'name': f"Build #{build['run']['run_number']} (artifact archive)",
            'body': json.dumps(build, sort_keys=True),
            'draft': True, 'prerelease': True,
        })
        for path in Path(directory).iterdir():
            # No --clobber: a retry can only fill missing assets in our draft.
            current = api(repo, f"releases/{created['id']}")
            if not current['draft'] or archive_manifest(repo, current) != build:
                raise RuntimeError('Archive was changed during upload')
            gh('release', 'upload', archive_tag, str(path), '--repo', repo)
        saved = api(repo, f"releases/{created['id']}")
        if not archive_complete(build, saved):
            raise RuntimeError('Archive verification failed; nothing will be deleted')
    print(f'Verified six durable ZIP assets: {archive_tag}', flush=True)


def apply_deletions(repo, result):
    if len(result['keep']) != 2 or result['archive_required']:
        raise RuntimeError('Two verified durable build sets are required before deletion')
    runs = {r['id']: r for r in result['inventory']['runs']}
    checked_runs = {}
    for artifact in result['delete_artifacts']:
        run_id = artifact['workflow_run']['id']
        if run_id not in checked_runs:
            checked_runs[run_id] = api(repo, f'actions/runs/{run_id}')
        current = checked_runs[run_id]
        if (current['status'] != 'completed'
                or current['run_attempt'] != runs[run_id]['run_attempt']):
            raise RuntimeError('A candidate run was restarted; refusing further deletion')
        api(repo, f"actions/artifacts/{artifact['id']}", 'DELETE')


def summary(result):
    lines = ['Artifact maintenance', '']
    for build in result['keep']:
        run = build['run']
        lines.append(f"Keep complete run #{run['run_number']} ({run['id']}), {run['head_sha']}")
    lines.append(f"Durable archives needed: {len(result['archive_required'])}")
    lines.append(f"Candidate artifacts: {len(result['delete_artifacts'])}; "
                 f"{result['reclaim_bytes']} bytes ({result['reclaim_bytes'] / 2**30:.3f} GiB)")
    return '\n'.join(lines) + '\n'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo', default=os.environ.get('GITHUB_REPOSITORY'))
    parser.add_argument('--mode', choices=['dry-run', 'archive-only', 'apply'], default='dry-run')
    parser.add_argument('--report', type=Path, default=Path('artifact-plan.json'))
    args = parser.parse_args()
    if not args.repo or not re.fullmatch(r'[\w.-]+/[\w.-]+', args.repo):
        parser.error('--repo OWNER/REPO or GITHUB_REPOSITORY is required')
    result = plan(args.repo, inventory(args.repo))
    args.report.write_text(json.dumps(result, indent=2), encoding='utf-8')
    print(summary(result))
    if args.mode != 'dry-run':
        if len(result['keep']) != 2:
            raise RuntimeError('Fewer than two complete successful builds; nothing will be deleted')
        for build in result['archive_required']:
            ensure_archive(args.repo, build, result['inventory']['releases'])
        refreshed = plan(args.repo, inventory(args.repo))
        if [tag(b) for b in result['keep']] != [tag(b) for b in refreshed['keep']]:
            raise RuntimeError('Newest builds changed during backup; rerun maintenance')
        args.report.write_text(json.dumps(refreshed, indent=2), encoding='utf-8')
        if args.mode == 'apply':
            apply_deletions(args.repo, refreshed)
        result = refreshed
    if os.environ.get('GITHUB_STEP_SUMMARY'):
        with open(os.environ['GITHUB_STEP_SUMMARY'], 'a', encoding='utf-8') as output:
            output.write(f'```text\nMode: {args.mode}\n{summary(result)}```\n')
    print(f'Mode: {args.mode}. Full inventory and candidate IDs: {args.report}')


if __name__ == '__main__':
    main()
