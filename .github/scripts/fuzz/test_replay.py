"""Replay integration checks with real Go targets and a local S3 fixture.

Run with: python3 -m unittest discover -s .github/scripts/fuzz -v
Requires Go, Bash 4+, jq and no registry or S3 credentials.
"""

import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("replay-corpus.sh").resolve()
SEED = 'go test fuzz v1\nstring("%s")\n'


class ReplayTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.cache = tempfile.TemporaryDirectory(prefix="fuzz-replay-cache-")

    @classmethod
    def tearDownClass(cls):
        cls.cache.cleanup()

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="fuzz-replay-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.corpus = self.root / "s3"
        self.corpus.mkdir()
        self.work = self.root / "src"
        self.work.mkdir()
        self.report = self.root / "report"
        self.bin = self.root / "bin"
        self.bin.mkdir()
        (self.work / "go.mod").write_text("module example.com/replay\n\ngo 1.20\n")
        for package in ("a", "b"):
            directory = self.work / package
            directory.mkdir()
            (directory / "fuzz_test.go").write_text('''package fixture
import "testing"
func FuzzSame(f *testing.F) {
    f.Add("ok")
    f.Fuzz(func(t *testing.T, s string) {
        if s == "crash" { t.Fatal("fixture crash") }
    })
}
''')
        self.command("task", '''#!/usr/bin/env bash
set -eu
[[ "$1" == fuzz:list ]]
[[ "${EMPTY_TARGETS:-}" != true ]] || exit 0
exec go test -vet=off ./... -list '^Fuzz'
''')
        self.command("aws", '''#!/usr/bin/env python3
import os, pathlib, shutil, sys
args = sys.argv[1:]
if args[0] == "configure":
    sys.exit(0)
pathlib.Path(os.environ["S3_REQUEST"]).write_text(" ".join(args))
if os.environ.get("S3_ERROR"):
    print("AccessDenied: fixture S3 error", file=sys.stderr)
    sys.exit(23)
assert args[2:4] == ["s3", "sync"], args
shutil.copytree(os.environ["S3_FIXTURE"], args[5], dirs_exist_ok=True)
''')
        self.env = {
            **os.environ,
            "PATH": str(self.bin) + os.pathsep + os.environ["PATH"],
            "GOTOOLCHAIN": "local",
            "GOPROXY": "off",
            "GOSUMDB": "off",
            "GOCACHE": self.cache.name,
            "GOWORK": "off",
            "S3_FIXTURE": str(self.corpus),
            "S3_REQUEST": str(self.root / "s3-request"),
            "FUZZ_REPLAY_REPORT_DIR": str(self.report),
            "FUZZ_REPLAY_IMAGE": "fixture/image:commit",
            "FUZZ_COMMIT": "fixture-commit",
            "FUZZ_RUN_URL": "https://example.com/actions/runs/1",
            "FUZZ_S3_ENDPOINT": "https://s3.example.com",
            "FUZZ_S3_ACCESS_KEY": "fixture",
            "FUZZ_S3_SECRET_KEY": "fixture",
            "FUZZ_S3_REPOSITORY": "deckhouse",
            "FUZZ_S3_BRANCH_SLUG": "main",
            "FUZZ_S3_COMPONENT": "./registry/syncer-fuzz",
        }

    def command(self, name, content):
        path = self.bin / name
        path.write_text(content)
        path.chmod(0o755)

    def seed(self, relative, value="ok"):
        path = self.corpus / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(SEED % value)

    def replay(self, expected_status=0):
        result = subprocess.run(
            [shutil.which("bash"), str(SCRIPT)], cwd=self.work, env=self.env,
            text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=120,
        )
        self.assertEqual(result.returncode, expected_status, result.stdout)
        return (self.report / "summary.txt").read_text()

    def test_empty_corpus_replays_builtin_seeds(self):
        summary = self.replay()
        self.assertIn("Targets checked: 2; failed: 0", summary)
        request = (self.root / "s3-request").read_text()
        self.assertIn("s3://anomaloys-materials/deckhouse/main/registry/syncer/", request)

    def test_all_corpus_formats_and_existing_seed(self):
        destination = self.work / "a/testdata/fuzz/FuzzSame"
        destination.mkdir(parents=True)
        (destination / "existing").write_text(SEED % "ok")
        self.seed("FuzzSame/corpus/minimized/existing", "crash")
        self.seed("FuzzSame/corpus/minimized/minimized")
        self.seed("FuzzSame/corpus/recovery/recovery")
        self.seed("FuzzSame/corpus/gocache-fuzz/example.com/replay/a/FuzzSame/cache")
        self.replay()
        self.assertEqual({p.name for p in destination.iterdir()},
                         {"existing", "minimized", "recovery", "cache"})
        self.assertEqual((destination / "existing").read_text(), SEED % "ok")

    def test_duplicate_target_names_use_package_key(self):
        self.seed("example.com/replay/b/FuzzSame/corpus/minimized/crash", "crash")
        summary = self.replay(1)
        self.assertIn("FAILED: example.com/replay/b / FuzzSame", summary)
        self.assertNotIn("FAILED: example.com/replay/a", summary)
        self.assertIn("Targets checked: 2; failed: 1", summary)
        output = self.report / "2-FuzzSame"
        self.assertIn("fixture crash", (output / "replay.log").read_text())
        self.assertIn("FuzzSame/crash", (output / "failed-tests.txt").read_text())
        self.assertTrue((output / "corpus/crash").is_file())
        self.assertIn("--entrypoint go", summary)
        self.assertIn("fixture/image:commit", summary)

    def test_failure_does_not_skip_remaining_targets(self):
        self.seed("FuzzSame/corpus/recovery/crash", "crash")
        summary = self.replay(1)
        self.assertIn("FAILED: example.com/replay/a / FuzzSame", summary)
        self.assertIn("Targets checked: 2; failed: 1", summary)

    def test_s3_error_fails_and_keeps_diagnostics(self):
        self.env["S3_ERROR"] = "true"
        self.replay(23)
        self.assertIn("AccessDenied", (self.report / "s3.log").read_text())

    def test_empty_target_list_fails(self):
        self.env["EMPTY_TARGETS"] = "true"
        self.replay(1)
        self.assertTrue((self.report / "targets.log").is_file())


if __name__ == "__main__":
    unittest.main()
