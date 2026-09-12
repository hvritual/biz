import unittest
from ce03_test_events import summarize


def event(action, package='app', test=None, output=None):
    result = {'Action': action, 'Package': package}
    if test is not None:
        result['Test'] = test
    if output is not None:
        result['Output'] = output
    return result


class TestGoEventEvidence(unittest.TestCase):
    def test_no_test_package_is_not_a_skipped_test(self):
        result = summarize([event('pass', test='TestOK'),
                            event('output', 'cli', output='? cli [no test files]\n'),
                            event('skip', 'cli')])
        self.assertEqual(result['leaf_tests'], 1)
        self.assertEqual(result['skipped'], 0)
        self.assertEqual(result['no_test_packages'], 1)

    def test_actual_skipped_test_is_rejected(self):
        with self.assertRaisesRegex(ValueError, 'skipped_tests='):
            summarize([event('pass', test='TestOK'), event('skip', test='TestNeedsDB')])

    def test_unexplained_package_skip_is_rejected(self):
        with self.assertRaisesRegex(ValueError, 'unexplained_package_skips='):
            summarize([event('pass', test='TestOK'), event('skip', 'unexplained')])

    def test_package_and_test_failures_are_rejected(self):
        for failure in (event('fail'), event('fail', test='TestBad')):
            with self.subTest(failure=failure), self.assertRaises(ValueError):
                summarize([event('pass', test='TestOK'), failure])

    def test_zero_executed_tests_are_rejected(self):
        with self.assertRaises(ValueError):
            summarize([event('output', output='? app [no test files]\n'), event('skip')])

    def test_leaf_counts_are_scoped_to_package(self):
        result = summarize([event('pass', 'a', 'TestOne'), event('pass', 'a', 'TestOne/child'),
                            event('pass', 'b', 'TestOne')])
        self.assertEqual(result['leaf_tests'], 2)
        self.assertEqual(result['pass_events'], 3)


if __name__ == '__main__':
    unittest.main()
