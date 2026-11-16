import copy
import unittest
from unittest.mock import patch

import maintain_artifacts as maintenance


def state():
    runs = [{'id': n, 'run_number': n, 'run_attempt': 1, 'status': 'completed',
             'conclusion': 'success'} for n in range(1, 5)]
    artifacts = [{'id': n * 100 + i, 'name': name, 'size_in_bytes': 123,
                  'expired': False, 'workflow_run': {'id': n}}
                 for n in range(1, 5) for i, name in enumerate(sorted(maintenance.TARGETS))]
    return runs, artifacts


class RetentionTests(unittest.TestCase):
    def test_keep_two_complete_runs(self):
        runs, artifacts = state()
        result = maintenance.plan(runs, artifacts)
        self.assertEqual([b['run']['id'] for b in result['keep']], [4, 3])
        self.assertEqual({a['workflow_run']['id'] for a in result['delete_artifacts']}, {1, 2})
        self.assertEqual(result['reclaim_bytes'], 12 * 123)

    def test_invalid_newer_build_never_replaces_full_set(self):
        for invalid in ('failure', 'cancelled', 'in_progress', 'missing', 'duplicate', 'expired'):
            with self.subTest(invalid=invalid):
                runs, artifacts = state()
                if invalid in ('failure', 'cancelled'):
                    runs[-1]['conclusion'] = invalid
                elif invalid == 'in_progress':
                    runs[-1]['status'] = invalid
                elif invalid == 'missing':
                    artifacts.pop()
                elif invalid == 'duplicate':
                    artifacts.append(copy.deepcopy(artifacts[-1]))
                else:
                    artifacts[-1]['expired'] = True
                result = maintenance.plan(runs, artifacts)
                self.assertEqual([b['run']['id'] for b in result['keep']], [3, 2])
                self.assertEqual({a['workflow_run']['id'] for a in result['delete_artifacts']}, {1})

    def test_unknown_artifacts_other_workflows_and_running_history_are_preserved(self):
        runs, artifacts = state()
        runs[0]['status'] = 'in_progress'
        artifacts += [{'id': 9990, 'name': 'report', 'workflow_run': {'id': 2}},
                      {'id': 9991, 'name': next(iter(maintenance.TARGETS)), 'workflow_run': {'id': 999}}]
        result = maintenance.plan(runs, artifacts)
        self.assertEqual(len(result['delete_artifacts']), 6)
        self.assertTrue(all(a['workflow_run']['id'] == 2 for a in result['delete_artifacts']))

    def test_fewer_than_two_sets_blocks_deletion(self):
        runs, artifacts = state()
        for run in runs[1:]:
            run['conclusion'] = 'failure'
        result = maintenance.plan(runs, artifacts)
        self.assertEqual(result['delete_artifacts'], [])
        with patch.object(maintenance, 'api') as api:
            with self.assertRaises(RuntimeError):
                maintenance.apply_deletions('owner/repo', result)
            api.assert_not_called()

    def test_only_actions_artifacts_can_be_deleted(self):
        runs, artifacts = state()
        result = maintenance.plan(runs, artifacts)
        result['inventory'] = {'runs': runs}
        with patch.object(maintenance, 'api', side_effect=lambda repo, endpoint, method='GET':
                          next(r for r in runs if endpoint == f"actions/runs/{r['id']}")
                          if method == 'GET' else None) as api:
            maintenance.apply_deletions('owner/repo', result)
            writes = [c for c in api.call_args_list if len(c.args) == 3]
            self.assertEqual(len(writes), 12)
            self.assertTrue(all(c.args[1].startswith('actions/artifacts/')
                                and c.args[2] == 'DELETE' for c in writes))

    def test_restarted_candidate_or_retained_run_blocks_deletion(self):
        for restarted in (1, 3, 4):
            with self.subTest(restarted=restarted):
                runs, artifacts = state()
                result = maintenance.plan(runs, artifacts)
                result['inventory'] = {'runs': runs}

                def current_run(repo, endpoint, method='GET'):
                    if method == 'GET':
                        run = next(r for r in runs if endpoint == f"actions/runs/{r['id']}")
                        return {**run, 'run_attempt': 2} if run['id'] == restarted else run

                with patch.object(maintenance, 'api', side_effect=current_run) as api:
                    with self.assertRaises(RuntimeError):
                        maintenance.apply_deletions('owner/repo', result)
                    self.assertFalse(any(len(c.args) == 3 for c in api.call_args_list))


if __name__ == '__main__':
    unittest.main()
