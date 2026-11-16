"""Delete old Release artifacts, keeping two complete successful build sets."""

import argparse
import json
import os
from pathlib import Path
import re
import subprocess


TARGETS = {
    'courseforge-linux-amd64', 'courseforge-linux-arm64',
    'courseforge-macos-amd64', 'courseforge-macos-arm64',
    'courseforge-windows-amd64.exe', 'courseforge-windows-arm64.exe',
}


def api(repo, endpoint, method='GET'):
    data = subprocess.check_output(
        ['gh', 'api', f'repos/{repo}/{endpoint}', '--method', method], timeout=300,
    )
    return json.loads(data) if data else None


def pages(repo, endpoint, key):
    data = subprocess.check_output([
        'gh', 'api', f'repos/{repo}/{endpoint}?per_page=100', '--paginate', '--slurp',
    ], timeout=300)
    return [item for page in json.loads(data) for item in page[key]]


def plan(runs, artifacts):
    runs = {r['id']: r for r in runs}
    complete = []
    for run in runs.values():
        files = [a for a in artifacts if (a.get('workflow_run') or {}).get('id') == run['id']
                 and a['name'] in TARGETS]
        if (run['status'] == 'completed' and run['conclusion'] == 'success'
                and len(files) == len(TARGETS) and {a['name'] for a in files} == TARGETS
                and all(not a['expired'] and a['size_in_bytes'] > 0 for a in files)):
            complete.append({'run': run, 'artifacts': files})
    keep = sorted(complete, key=lambda b: b['run']['run_number'], reverse=True)[:2]
    protected = {b['run']['id'] for b in keep}
    delete = []
    if len(keep) == 2:
        for artifact in artifacts:
            run = runs.get((artifact.get('workflow_run') or {}).get('id'))
            if (artifact['name'] in TARGETS | {'courseforge-windows-amd64'} and run
                    and run['status'] == 'completed' and run['id'] not in protected
                    and run['run_number'] < keep[-1]['run']['run_number']):
                delete.append(artifact)
    return {'keep': keep, 'delete_artifacts': delete,
            'reclaim_bytes': sum(a['size_in_bytes'] for a in delete)}


def apply_deletions(repo, result):
    if len(result['keep']) != 2:
        raise RuntimeError('Fewer than two complete successful sets; nothing will be deleted')
    for build in result['keep']:
        run = build['run']
        current = api(repo, f"actions/runs/{run['id']}")
        if (current['status'] != 'completed' or current['conclusion'] != 'success'
                or current['run_attempt'] != run['run_attempt']):
            raise RuntimeError('Retained run changed; nothing will be deleted')
    runs = {r['id']: r for r in result['inventory']['runs']}
    checked = {}
    for artifact in result['delete_artifacts']:
        run_id = artifact['workflow_run']['id']
        if run_id not in checked:
            checked[run_id] = api(repo, f'actions/runs/{run_id}')
        if (checked[run_id]['status'] != 'completed'
                or checked[run_id]['run_attempt'] != runs[run_id]['run_attempt']):
            raise RuntimeError('Candidate run restarted; refusing further deletion')
        api(repo, f"actions/artifacts/{artifact['id']}", 'DELETE')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo', default=os.environ.get('GITHUB_REPOSITORY'))
    parser.add_argument('--mode', choices=['dry-run', 'apply'], default='dry-run')
    parser.add_argument('--report', type=Path, default=Path('artifact-plan.json'))
    args = parser.parse_args()
    if not args.repo or not re.fullmatch(r'[\w.-]+/[\w.-]+', args.repo):
        parser.error('--repo OWNER/REPO or GITHUB_REPOSITORY is required')
    runs = pages(args.repo, 'actions/workflows/release.yml/runs', 'workflow_runs')
    artifacts = pages(args.repo, 'actions/artifacts', 'artifacts')
    result = plan(runs, artifacts)
    result.update(repository=args.repo, inventory={'runs': runs, 'artifacts': artifacts})
    args.report.write_text(json.dumps(result, indent=2), encoding='utf-8')
    summary = [f'Mode: {args.mode}']
    summary += [f"Keep run #{b['run']['run_number']} ({b['run']['id']}): six platforms"
                for b in result['keep']]
    summary.append(f"Delete candidates: {len(result['delete_artifacts'])}; "
                   f"{result['reclaim_bytes']} bytes. Full list: {args.report}")
    print('\n'.join(summary))
    if args.mode == 'apply':
        apply_deletions(args.repo, result)
    if os.environ.get('GITHUB_STEP_SUMMARY'):
        with open(os.environ['GITHUB_STEP_SUMMARY'], 'a', encoding='utf-8') as output:
            output.write('```text\n' + '\n'.join(summary) + '\n```\n')


if __name__ == '__main__':
    main()
