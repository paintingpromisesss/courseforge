import copy
import hashlib
import io
import unittest
from unittest.mock import patch

import maintain_artifacts as maintenance


REPO = 'paintingpromisesss/courseforge'


def run(number, conclusion='success', status='completed'):
    return {
        'id': number, 'run_number': number, 'run_attempt': 1,
        'head_sha': f'{number:040x}', 'head_branch': 'main',
        'created_at': f'2026-10-{number:02}T00:00:00Z',
        'status': status, 'conclusion': conclusion,
    }


def artifacts(number):
    return [{
        'id': number * 100 + i, 'name': name, 'size_in_bytes': 123,
        'digest': 'sha256:' + f'{number * 100 + i:064x}',
        'expired': False, 'workflow_run': {'id': number},
    } for i, name in enumerate(maintenance.TARGETS)]


def state(*numbers):
    return {'runs': [run(n) for n in numbers],
            'artifacts': [a for n in numbers for a in artifacts(n)], 'releases': []}


def archive(number, draft=True):
    build = maintenance.manifest(REPO, run(number), artifacts(number))
    return {
        'id': number + 1000, 'tag_name': maintenance.tag(build), 'draft': draft,
        'body': maintenance.json.dumps(build),
        'assets': [{'name': a['name'] + '.zip', 'size': a['size_in_bytes'],
                    'state': 'uploaded', 'digest': a['digest']} for a in artifacts(number)],
    }


class RetentionTests(unittest.TestCase):
    def test_two_complete_runs_not_two_files_per_platform(self):
        data = state(1, 2, 3)
        data['runs'].append(run(4))
        data['artifacts'] += artifacts(4)[:-1]
        result = maintenance.plan(REPO, data)
        self.assertEqual([b['run']['id'] for b in result['keep']], [3, 2])
        self.assertEqual({a['workflow_run']['id'] for a in result['delete_artifacts']}, {1})
        self.assertEqual(result['reclaim_bytes'], 6 * 123)

    def test_failed_cancelled_and_running_builds_never_displace_success(self):
        data = state(1, 2, 3, 4, 5)
        data['runs'][2]['conclusion'] = 'failure'
        data['runs'][3]['conclusion'] = 'cancelled'
        data['runs'][4].update(status='in_progress', conclusion=None)
        result = maintenance.plan(REPO, data)
        self.assertEqual([b['run']['id'] for b in result['keep']], [2, 1])
        self.assertEqual(result['delete_artifacts'], [])

    def test_completed_partial_and_failed_history_can_be_removed(self):
        data = state(1, 2, 3, 4)
        data['runs'][0]['conclusion'] = 'cancelled'
        data['artifacts'] = [a for a in data['artifacts'] if a['id'] != 100]
        result = maintenance.plan(REPO, data)
        self.assertEqual({a['workflow_run']['id'] for a in result['delete_artifacts']}, {1, 2})

    def test_fewer_than_two_builds_blocks_all_deletion(self):
        data = state(1, 2)
        data['artifacts'][-1]['expired'] = True
        result = maintenance.plan(REPO, data)
        self.assertEqual(result['delete_artifacts'], [])
        with self.assertRaisesRegex(RuntimeError, 'Two verified'):
            maintenance.apply_deletions(REPO, result)

    def test_unknown_names_and_other_workflows_are_untouched(self):
        data = state(1, 2, 3)
        foreign = {'id': 9000, 'name': maintenance.TARGETS[0], 'workflow_run': {'id': 999}}
        unknown = {'id': 9001, 'name': 'test-report', 'workflow_run': {'id': 1}}
        data['artifacts'] += [foreign, unknown]
        ids = {a['id'] for a in maintenance.plan(REPO, data)['delete_artifacts']}
        self.assertNotIn(9000, ids)
        self.assertNotIn(9001, ids)

    def test_missing_checksum_or_duplicate_platform_is_not_complete(self):
        data = state(1, 2, 3)
        data['artifacts'][-1]['digest'] = None
        self.assertEqual([b['run']['id'] for b in maintenance.plan(REPO, data)['keep']], [2, 1])
        data = state(1, 2, 3)
        data['artifacts'].append(copy.deepcopy(data['artifacts'][-1]))
        self.assertEqual([b['run']['id'] for b in maintenance.plan(REPO, data)['keep']], [2, 1])

    def test_archives_survive_expired_artifacts_and_removed_run_history(self):
        data = state(3)
        data['releases'] = [archive(1), archive(2)]
        result = maintenance.plan(REPO, data)
        self.assertEqual([b['run']['id'] for b in result['keep']], [3, 2])
        self.assertEqual([b['run']['id'] for b in result['archive_required']], [3])
        self.assertNotIn('delete_drafts', result)
        data['artifacts'] = []
        data['runs'] = []
        result = maintenance.plan(REPO, data)
        self.assertEqual([b['run']['id'] for b in result['keep']], [2, 1])
        self.assertEqual(result['archive_required'], [])

    def test_incomplete_corrupt_and_unrelated_archives_are_not_backups(self):
        for change in ('missing', 'digest', 'size', 'body', 'tag'):
            with self.subTest(change=change):
                release = archive(3)
                if change == 'missing':
                    release['assets'].pop()
                elif change == 'digest':
                    release['assets'][0]['digest'] = 'sha256:' + '0' * 64
                elif change == 'size':
                    release['assets'][0]['size'] += 1
                elif change == 'body':
                    release['body'] = '{}'
                else:
                    release['tag_name'] = 'v1.0.0'
                data = state(1, 2)
                data['releases'] = [release]
                self.assertEqual([b['run']['id'] for b in maintenance.plan(REPO, data)['keep']], [2, 1])

    def test_published_releases_never_enter_deletion_plan(self):
        data = state(2, 3)
        data['releases'] = [archive(1, draft=False)]
        result = maintenance.plan(REPO, data)
        self.assertNotIn('delete_drafts', result)

    def test_no_deletions_until_both_archives_are_verified(self):
        with patch.object(maintenance, 'api') as api:
            with self.assertRaisesRegex(RuntimeError, 'Two verified'):
                maintenance.apply_deletions(REPO, maintenance.plan(REPO, state(1, 2, 3)))
            api.assert_not_called()

    def test_restarted_candidate_run_aborts_before_delete(self):
        data = state(1, 2, 3)
        data['releases'] = [archive(2), archive(3)]
        with patch.object(maintenance, 'api', return_value=run(1, status='in_progress')) as api:
            with self.assertRaisesRegex(RuntimeError, 'restarted'):
                maintenance.apply_deletions(REPO, maintenance.plan(REPO, data))
            self.assertEqual(api.call_count, 1)

    def test_deletion_only_calls_actions_artifact_endpoints(self):
        data = state(1, 2, 3)
        data['releases'] = [archive(1), archive(2), archive(3)]
        with patch.object(maintenance, 'api', return_value=run(1)) as api:
            maintenance.apply_deletions(REPO, maintenance.plan(REPO, data))
            deleted = [call for call in api.call_args_list if len(call.args) > 2]
            self.assertEqual(len(deleted), 6)
            self.assertTrue(all(call.args[1].startswith('actions/artifacts/')
                                and call.args[2] == 'DELETE' for call in deleted))

    def test_existing_verified_archive_needs_no_uploads(self):
        release = archive(3)
        build = maintenance.archive_manifest(REPO, release)
        with patch.object(maintenance, 'gh') as gh, patch.object(maintenance, 'api') as api:
            maintenance.ensure_archive(REPO, build, [release])
            gh.assert_not_called()
            api.assert_not_called()

    def archive_upload(self, corrupt_download=False, incomplete_existing=False):
        source = artifacts(3)
        files = {}
        for artifact in source:
            output = io.BytesIO()
            with maintenance.zipfile.ZipFile(output, 'w') as archive_file:
                archive_file.writestr(artifact['name'], b'binary')
            data = output.getvalue()
            files[artifact['id']] = data
            artifact.update(size_in_bytes=len(data), digest='sha256:' + hashlib.sha256(data).hexdigest())
        build = maintenance.manifest(REPO, run(3), source)
        created = {}
        writes = []

        def fake_api(repo, endpoint, method='GET', payload=None):
            if method == 'POST':
                writes.append((endpoint, payload))
                created.update(payload, id=9999, assets=[])
            return created

        def fake_gh(*args, output=None):
            if args[0] == 'api':
                artifact_id = int(args[1].split('/')[-2])
                output.write(b'bad download' if corrupt_download else files[artifact_id])
            else:
                path = maintenance.Path(args[3])
                data = path.read_bytes()
                created['assets'].append({
                    'name': path.name, 'state': 'uploaded', 'size': len(data),
                    'digest': 'sha256:' + hashlib.sha256(data).hexdigest(),
                })

        existing = []
        if incomplete_existing:
            broken = archive(3)
            broken['assets'][0].update(state='starter', size=0)
            existing = [broken]
        with patch.object(maintenance, 'gh', side_effect=fake_gh), \
                patch.object(maintenance, 'api', side_effect=fake_api):
            if corrupt_download:
                with self.assertRaisesRegex(RuntimeError, 'checksum/size mismatch'):
                    maintenance.ensure_archive(REPO, build, existing)
            else:
                maintenance.ensure_archive(REPO, build, existing)
        return writes, created

    def test_upload_verifies_all_six_original_zip_files(self):
        writes, created = self.archive_upload()
        self.assertEqual(len(writes), 1)
        self.assertEqual(writes[0][0], 'releases')
        self.assertTrue(created['draft'])
        self.assertEqual(len(created['assets']), 6)

    def test_bad_download_prevents_any_remote_writes(self):
        writes, _ = self.archive_upload(corrupt_download=True)
        self.assertEqual(writes, [])

    def test_starter_asset_is_left_untouched_and_retry_uses_new_draft(self):
        writes, created = self.archive_upload(incomplete_existing=True)
        self.assertEqual(len(writes), 1)
        self.assertTrue(created['tag_name'].startswith(maintenance.PREFIX + '3-1-retry-'))
        self.assertIsNotNone(maintenance.archive_manifest(REPO, created))


if __name__ == '__main__':
    unittest.main()
